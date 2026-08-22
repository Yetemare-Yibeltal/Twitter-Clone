// backend/internal/adapter/storage.go
package adapter

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/h2non/filetype"
	"github.com/sirupsen/logrus"
	"golang.org/x/image/draw"

	"twitter-clone/backend/pkg/logger"
)

// StorageProvider defines the interface for file storage operations.
type StorageProvider interface {
	Upload(ctx context.Context, file io.Reader, filename string, opts *UploadOptions) (*UploadResult, error)
	UploadMultipart(ctx context.Context, file io.Reader, filename string, opts *UploadOptions, chunkSize int64) (*UploadResult, error)
	Download(ctx context.Context, filename string, w io.Writer) error
	Delete(ctx context.Context, filename string) error
	DeleteBatch(ctx context.Context, filenames []string) error
	List(ctx context.Context, prefix string, limit int, marker string) (*ListResult, error)
	GetURL(ctx context.Context, filename string, expiry time.Duration) (string, error)
	GetMetadata(ctx context.Context, filename string) (*FileMetadata, error)
	SupportsTransformations() bool
	Transform(ctx context.Context, filename string, transform TransformOptions) (string, error)
	ProviderName() string
}

// UploadOptions controls how a file is uploaded.
type UploadOptions struct {
	Public          bool
	ContentType     string
	CacheControl    string
	ACL             string
	Metadata        map[string]string
	Tags            map[string]string
	Transformations []Transformation
	Folder          string
}

// UploadResult contains the result of an upload.
type UploadResult struct {
	URL         string
	PublicID    string
	Format      string
	Width       int
	Height      int
	Bytes       int64
	ETag        string
	SecureURL   string
	Metadata    map[string]string
	CreatedAt   time.Time
	ResourceType string
}

// FileMetadata represents file metadata.
type FileMetadata struct {
	Name         string
	Size         int64
	ContentType  string
	LastModified time.Time
	ETag         string
	Metadata     map[string]string
	PublicURL    string
	IsImage      bool
	Width        int
	Height       int
}

// ListResult holds paginated file listing.
type ListResult struct {
	Files    []*FileMetadata
	NextMarker string
	IsTruncated bool
}

// TransformOptions describes image transformations.
type TransformOptions struct {
	Width     int
	Height    int
	Crop      string // "fill", "fit", "limit", "pad"
	Gravity   string // "center", "north", etc.
	Quality   int
	Format    string // "jpg", "png", "webp"
	Rotate    int
	Effect    string // "blur", "grayscale", etc.
}

// Transformation is a single transformation step.
type Transformation struct {
	Type  string // "resize", "crop", "rotate", "format", "quality"
	Value interface{}
}

// StorageAdapter is the main interface for storage operations.
type StorageAdapter interface {
	StorageProvider
	// Additional convenience methods
	UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, opts *UploadOptions) (*UploadResult, error)
	UploadBuffer(ctx context.Context, data []byte, filename string, opts *UploadOptions) (*UploadResult, error)
	Exists(ctx context.Context, filename string) (bool, error)
	Copy(ctx context.Context, src, dst string) error
	Move(ctx context.Context, src, dst string) error
	GetThumbnail(ctx context.Context, filename string, size int) (string, error)
	Close() error
}

// Config holds storage provider configuration.
type StorageConfig struct {
	Provider    string            // "local", "cloudinary", "s3"
	LocalPath   string            // for "local"
	Cloudinary  CloudinaryConfig  // for "cloudinary"
	S3          S3Config          // for "s3"
	DefaultACL  string
	MaxFileSize int64
	AllowedMime map[string]bool
}

// CloudinaryConfig holds Cloudinary credentials.
type CloudinaryConfig struct {
	CloudName   string
	APIKey      string
	APISecret   string
	Secure      bool
	DefaultFolder string
}

// S3Config holds AWS S3 credentials.
type S3Config struct {
	Region     string
	Bucket     string
	AccessKey  string
	SecretKey  string
	Endpoint   string // optional, for S3-compatible
	PathStyle  bool
	UseSSL     bool
}

// localProvider implements StorageProvider for local filesystem.
type localProvider struct {
	basePath string
	log      *logrus.Entry
	mu       sync.RWMutex
}

// NewLocalProvider creates a new local filesystem provider.
func NewLocalProvider(basePath string) (StorageProvider, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}
	return &localProvider{
		basePath: basePath,
		log:      logger.WithField("provider", "local"),
	}, nil
}

func (p *localProvider) ProviderName() string { return "local" }

func (p *localProvider) SupportsTransformations() bool { return false }

func (p *localProvider) Upload(ctx context.Context, file io.Reader, filename string, opts *UploadOptions) (*UploadResult, error) {
	// Sanitize filename
	filename = sanitizeFilename(filename)
	if opts != nil && opts.Folder != "" {
		filename = filepath.Join(opts.Folder, filename)
	}
	fullPath := filepath.Join(p.basePath, filename)
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}
	// Create file
	f, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()
	// Copy data
	written, err := io.Copy(f, file)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}
	// Determine content type
	contentType := opts.ContentType
	if contentType == "" {
		// Try to detect
		detected, err := detectContentType(fullPath)
		if err == nil {
			contentType = detected
		} else {
			contentType = "application/octet-stream"
		}
	}
	// Get file info
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	return &UploadResult{
		URL:       "/uploads/" + filename,
		PublicID:  filename,
		Bytes:     written,
		ETag:      fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))),
		CreatedAt: info.ModTime(),
		Metadata:  opts.Metadata,
	}, nil
}

func (p *localProvider) UploadMultipart(ctx context.Context, file io.Reader, filename string, opts *UploadOptions, chunkSize int64) (*UploadResult, error) {
	// For local, we just stream upload (no actual multipart)
	return p.Upload(ctx, file, filename, opts)
}

func (p *localProvider) Download(ctx context.Context, filename string, w io.Writer) error {
	fullPath := filepath.Join(p.basePath, filename)
	f, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %w", err)
		}
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

func (p *localProvider) Delete(ctx context.Context, filename string) error {
	fullPath := filepath.Join(p.basePath, filename)
	return os.Remove(fullPath)
}

func (p *localProvider) DeleteBatch(ctx context.Context, filenames []string) error {
	var errs []string
	for _, f := range filenames {
		if err := p.Delete(ctx, f); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", f, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("batch delete errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (p *localProvider) List(ctx context.Context, prefix string, limit int, marker string) (*ListResult, error) {
	fullPrefix := filepath.Join(p.basePath, prefix)
	var files []*FileMetadata
	entries, err := os.ReadDir(fullPrefix)
	if err != nil {
		if os.IsNotExist(err) {
			return &ListResult{Files: []*FileMetadata{}}, nil
		}
		return nil, err
	}
	started := marker == ""
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !started && entry.Name() == marker {
			started = true
			continue
		}
		if !started {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, &FileMetadata{
			Name:         entry.Name(),
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})
		if limit > 0 && len(files) >= limit {
			break
		}
	}
	var nextMarker string
	if len(files) > 0 && len(files) == limit {
		nextMarker = files[len(files)-1].Name
	}
	return &ListResult{
		Files:       files,
		NextMarker:  nextMarker,
		IsTruncated: nextMarker != "",
	}, nil
}

func (p *localProvider) GetURL(ctx context.Context, filename string, expiry time.Duration) (string, error) {
	// For local, we return a relative path (served by a static route)
	return "/uploads/" + filename, nil
}

func (p *localProvider) GetMetadata(ctx context.Context, filename string) (*FileMetadata, error) {
	fullPath := filepath.Join(p.basePath, filename)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found")
		}
		return nil, err
	}
	return &FileMetadata{
		Name:         filename,
		Size:         info.Size(),
		LastModified: info.ModTime(),
		ContentType:  "application/octet-stream", // Could detect
	}, nil
}

func (p *localProvider) Transform(ctx context.Context, filename string, transform TransformOptions) (string, error) {
	return "", fmt.Errorf("local provider does not support transformations")
}

// cloudinaryProvider implements StorageProvider for Cloudinary.
type cloudinaryProvider struct {
	cld  *cloudinary.Cloudinary
	cfg  CloudinaryConfig
	log  *logrus.Entry
	mu   sync.RWMutex
}

// NewCloudinaryProvider creates a new Cloudinary provider.
func NewCloudinaryProvider(cfg CloudinaryConfig) (StorageProvider, error) {
	cld, err := cloudinary.NewFromParams(cfg.CloudName, cfg.APIKey, cfg.APISecret)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Cloudinary: %w", err)
	}
	return &cloudinaryProvider{
		cld: cld,
		cfg: cfg,
		log: logger.WithField("provider", "cloudinary"),
	}, nil
}

func (p *cloudinaryProvider) ProviderName() string { return "cloudinary" }

func (p *cloudinaryProvider) SupportsTransformations() bool { return true }

func (p *cloudinaryProvider) Upload(ctx context.Context, file io.Reader, filename string, opts *UploadOptions) (*UploadResult, error) {
	// Determine public ID
	publicID := strings.TrimSuffix(filename, filepath.Ext(filename))
	if opts != nil && opts.Folder != "" {
		publicID = filepath.Join(opts.Folder, publicID)
	}
	uploadParams := uploader.UploadParams{
		PublicID:   publicID,
		Folder:     opts.Folder,
		UniqueFilename: aws.Bool(false),
		Overwrite:  aws.Bool(true),
	}
	if opts != nil && opts.Metadata != nil {
		uploadParams.Context = opts.Metadata
	}
	// Read file into buffer (Cloudinary requires seeking)
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, file); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	resp, err := p.cld.Upload.Upload(ctx, buf, uploadParams)
	if err != nil {
		return nil, fmt.Errorf("cloudinary upload failed: %w", err)
	}
	return &UploadResult{
		URL:       resp.URL,
		SecureURL: resp.SecureURL,
		PublicID:  resp.PublicID,
		Format:    resp.Format,
		Width:     resp.Width,
		Height:    resp.Height,
		Bytes:     int64(resp.Bytes),
		ETag:      resp.ETag,
		CreatedAt: resp.CreatedAt,
		Metadata:  resp.Context,
		ResourceType: resp.ResourceType,
	}, nil
}

func (p *cloudinaryProvider) UploadMultipart(ctx context.Context, file io.Reader, filename string, opts *UploadOptions, chunkSize int64) (*UploadResult, error) {
	// Cloudinary doesn't support true multipart; we just upload whole.
	return p.Upload(ctx, file, filename, opts)
}

func (p *cloudinaryProvider) Download(ctx context.Context, filename string, w io.Writer) error {
	// For Cloudinary, we need to fetch the file using the URL.
	url, err := p.GetURL(ctx, filename, 0)
	if err != nil {
		return err
	}
	// We could do HTTP GET, but for simplicity we return error.
	// For production, implement HTTP client download.
	return fmt.Errorf("download not implemented for Cloudinary; use URL")
}

func (p *cloudinaryProvider) Delete(ctx context.Context, filename string) error {
	resp, err := p.cld.Upload.Destroy(ctx, uploader.DestroyParams{PublicID: filename})
	if err != nil {
		return err
	}
	if resp.Result != "ok" {
		return fmt.Errorf("cloudinary delete failed: %s", resp.Result)
	}
	return nil
}

func (p *cloudinaryProvider) DeleteBatch(ctx context.Context, filenames []string) error {
	var errs []string
	for _, f := range filenames {
		if err := p.Delete(ctx, f); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", f, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("batch delete errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (p *cloudinaryProvider) List(ctx context.Context, prefix string, limit int, marker string) (*ListResult, error) {
	// Cloudinary does not have a simple list; use management API.
	// For brevity, implement a stub that returns an error.
	return nil, fmt.Errorf("list not implemented for Cloudinary")
}

func (p *cloudinaryProvider) GetURL(ctx context.Context, filename string, expiry time.Duration) (string, error) {
	// For Cloudinary, return the secure URL with optional transformations.
	return p.cld.URL.Public(filename).Secure(true).String(), nil
}

func (p *cloudinaryProvider) GetMetadata(ctx context.Context, filename string) (*FileMetadata, error) {
	// Resource API call would be needed.
	return nil, fmt.Errorf("metadata not implemented for Cloudinary")
}

func (p *cloudinaryProvider) Transform(ctx context.Context, filename string, transform TransformOptions) (string, error) {
	// Build transformation string
	trans := p.cld.URL.Public(filename)
	if transform.Width > 0 || transform.Height > 0 {
		var crop string
		if transform.Crop != "" {
			crop = transform.Crop
		} else {
			crop = "fill"
		}
		w := transform.Width
		h := transform.Height
		if w == 0 {
			w = h
		}
		if h == 0 {
			h = w
		}
		trans = trans.Transform(fmt.Sprintf("c_%s,w_%d,h_%d", crop, w, h))
	}
	if transform.Quality > 0 {
		trans = trans.Transform(fmt.Sprintf("q_%d", transform.Quality))
	}
	if transform.Format != "" {
		trans = trans.Transform(fmt.Sprintf("f_%s", transform.Format))
	}
	if transform.Rotate > 0 {
		trans = trans.Transform(fmt.Sprintf("a_%d", transform.Rotate))
	}
	if transform.Effect != "" {
		trans = trans.Transform("e_" + transform.Effect)
	}
	return trans.Secure(true).String(), nil
}

// s3Provider implements StorageProvider for AWS S3.
type s3Provider struct {
	s3     *s3.S3
	bucket string
	cfg    S3Config
	log    *logrus.Entry
	mu     sync.RWMutex
}

// NewS3Provider creates a new S3 provider.
func NewS3Provider(cfg S3Config) (StorageProvider, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(cfg.Region),
		Credentials:      credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, ""),
		Endpoint:         aws.String(cfg.Endpoint),
		S3ForcePathStyle: aws.Bool(cfg.PathStyle),
		DisableSSL:       aws.Bool(!cfg.UseSSL),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}
	svc := s3.New(sess)
	return &s3Provider{
		s3:     svc,
		bucket: cfg.Bucket,
		cfg:    cfg,
		log:    logger.WithField("provider", "s3"),
	}, nil
}

func (p *s3Provider) ProviderName() string { return "s3" }

func (p *s3Provider) SupportsTransformations() bool { return false }

func (p *s3Provider) Upload(ctx context.Context, file io.Reader, filename string, opts *UploadOptions) (*UploadResult, error) {
	// Use uploader
	uploader := s3manager.NewUploaderWithClient(p.s3)
	input := &s3manager.UploadInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(filename),
		Body:   file,
	}
	if opts != nil {
		if opts.ContentType != "" {
			input.ContentType = aws.String(opts.ContentType)
		}
		if opts.CacheControl != "" {
			input.CacheControl = aws.String(opts.CacheControl)
		}
		if opts.ACL != "" {
			input.ACL = aws.String(opts.ACL)
		}
		if opts.Metadata != nil {
			metadata := make(map[string]*string)
			for k, v := range opts.Metadata {
				metadata[k] = aws.String(v)
			}
			input.Metadata = metadata
		}
	}
	result, err := uploader.Upload(input)
	if err != nil {
		return nil, fmt.Errorf("s3 upload failed: %w", err)
	}
	return &UploadResult{
		URL:   result.Location,
		ETag:  *result.ETag,
		Bytes: 0, // not known
	}, nil
}

func (p *s3Provider) UploadMultipart(ctx context.Context, file io.Reader, filename string, opts *UploadOptions, chunkSize int64) (*UploadResult, error) {
	// For S3, we can just use uploader which handles multipart automatically.
	return p.Upload(ctx, file, filename, opts)
}

func (p *s3Provider) Download(ctx context.Context, filename string, w io.Writer) error {
	input := &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(filename),
	}
	result, err := p.s3.GetObjectWithContext(ctx, input)
	if err != nil {
		return err
	}
	defer result.Body.Close()
	_, err = io.Copy(w, result.Body)
	return err
}

func (p *s3Provider) Delete(ctx context.Context, filename string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(filename),
	}
	_, err := p.s3.DeleteObjectWithContext(ctx, input)
	return err
}

func (p *s3Provider) DeleteBatch(ctx context.Context, filenames []string) error {
	var objects []*s3.ObjectIdentifier
	for _, f := range filenames {
		objects = append(objects, &s3.ObjectIdentifier{Key: aws.String(f)})
	}
	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(p.bucket),
		Delete: &s3.Delete{
			Objects: objects,
			Quiet:   aws.Bool(true),
		},
	}
	_, err := p.s3.DeleteObjectsWithContext(ctx, input)
	return err
}

func (p *s3Provider) List(ctx context.Context, prefix string, limit int, marker string) (*ListResult, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(p.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int64(int64(limit)),
	}
	if marker != "" {
		input.StartAfter = aws.String(marker)
	}
	result, err := p.s3.ListObjectsV2WithContext(ctx, input)
	if err != nil {
		return nil, err
	}
	files := make([]*FileMetadata, 0, len(result.Contents))
	for _, obj := range result.Contents {
		files = append(files, &FileMetadata{
			Name:         *obj.Key,
			Size:         *obj.Size,
			LastModified: *obj.LastModified,
			ETag:         *obj.ETag,
		})
	}
	var nextMarker string
	if result.IsTruncated != nil && *result.IsTruncated {
		nextMarker = *result.NextContinuationToken
	}
	return &ListResult{
		Files:       files,
		NextMarker:  nextMarker,
		IsTruncated: result.IsTruncated != nil && *result.IsTruncated,
	}, nil
}

func (p *s3Provider) GetURL(ctx context.Context, filename string, expiry time.Duration) (string, error) {
	req, _ := p.s3.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(filename),
	})
	return req.Presign(expiry)
}

func (p *s3Provider) GetMetadata(ctx context.Context, filename string) (*FileMetadata, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(filename),
	}
	result, err := p.s3.HeadObjectWithContext(ctx, input)
	if err != nil {
		return nil, err
	}
	meta := make(map[string]string)
	for k, v := range result.Metadata {
		if v != nil {
			meta[k] = *v
		}
	}
	return &FileMetadata{
		Name:         filename,
		Size:         *result.ContentLength,
		ContentType:  *result.ContentType,
		LastModified: *result.LastModified,
		ETag:         *result.ETag,
		Metadata:     meta,
	}, nil
}

func (p *s3Provider) Transform(ctx context.Context, filename string, transform TransformOptions) (string, error) {
	return "", fmt.Errorf("s3 provider does not support transformations")
}

// storageAdapter implements StorageAdapter with additional methods.
type storageAdapter struct {
	provider StorageProvider
	cfg      StorageConfig
	log      *logrus.Entry
	mu       sync.RWMutex
}

// NewStorageAdapter creates a new storage adapter based on config.
func NewStorageAdapter(providerName string, config map[string]string) (StorageAdapter, error) {
	var provider StorageProvider
	var cfg StorageConfig

	switch providerName {
	case "local":
		localPath := config["LOCAL_PATH"]
		if localPath == "" {
			localPath = "./uploads"
		}
		p, err := NewLocalProvider(localPath)
		if err != nil {
			return nil, err
		}
		provider = p
		cfg.Provider = "local"
		cfg.LocalPath = localPath
	case "cloudinary":
		cloudCfg := CloudinaryConfig{
			CloudName:   config["CLOUD_NAME"],
			APIKey:      config["API_KEY"],
			APISecret:   config["API_SECRET"],
			Secure:      true,
		}
		p, err := NewCloudinaryProvider(cloudCfg)
		if err != nil {
			return nil, err
		}
		provider = p
		cfg.Provider = "cloudinary"
		cfg.Cloudinary = cloudCfg
	case "s3":
		s3Cfg := S3Config{
			Region:    config["REGION"],
			Bucket:    config["BUCKET"],
			AccessKey: config["ACCESS_KEY"],
			SecretKey: config["SECRET_KEY"],
			Endpoint:  config["ENDPOINT"],
			UseSSL:    config["USE_SSL"] != "false",
		}
		p, err := NewS3Provider(s3Cfg)
		if err != nil {
			return nil, err
		}
		provider = p
		cfg.Provider = "s3"
		cfg.S3 = s3Cfg
	default:
		return nil, fmt.Errorf("unknown storage provider: %s", providerName)
	}

	// Default max file size: 10MB
	if cfg.MaxFileSize == 0 {
		cfg.MaxFileSize = 10 << 20
	}
	// Allowed MIME types (optional)
	cfg.AllowedMime = map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
		"video/mp4":  true,
		"video/quicktime": true,
		"application/pdf": false, // block by default if not set
	}

	return &storageAdapter{
		provider: provider,
		cfg:      cfg,
		log:      logger.WithField("component", "storage_adapter"),
	}, nil
}

// Implement all StorageProvider methods by forwarding.
func (a *storageAdapter) Upload(ctx context.Context, file io.Reader, filename string, opts *UploadOptions) (*UploadResult, error) {
	return a.provider.Upload(ctx, file, filename, opts)
}

func (a *storageAdapter) UploadMultipart(ctx context.Context, file io.Reader, filename string, opts *UploadOptions, chunkSize int64) (*UploadResult, error) {
	return a.provider.UploadMultipart(ctx, file, filename, opts, chunkSize)
}

func (a *storageAdapter) Download(ctx context.Context, filename string, w io.Writer) error {
	return a.provider.Download(ctx, filename, w)
}

func (a *storageAdapter) Delete(ctx context.Context, filename string) error {
	return a.provider.Delete(ctx, filename)
}

func (a *storageAdapter) DeleteBatch(ctx context.Context, filenames []string) error {
	return a.provider.DeleteBatch(ctx, filenames)
}

func (a *storageAdapter) List(ctx context.Context, prefix string, limit int, marker string) (*ListResult, error) {
	return a.provider.List(ctx, prefix, limit, marker)
}

func (a *storageAdapter) GetURL(ctx context.Context, filename string, expiry time.Duration) (string, error) {
	return a.provider.GetURL(ctx, filename, expiry)
}

func (a *storageAdapter) GetMetadata(ctx context.Context, filename string) (*FileMetadata, error) {
	return a.provider.GetMetadata(ctx, filename)
}

func (a *storageAdapter) SupportsTransformations() bool {
	return a.provider.SupportsTransformations()
}

func (a *storageAdapter) Transform(ctx context.Context, filename string, transform TransformOptions) (string, error) {
	if !a.SupportsTransformations() {
		return "", fmt.Errorf("transformations not supported by provider")
	}
	return a.provider.Transform(ctx, filename, transform)
}

func (a *storageAdapter) ProviderName() string {
	return a.provider.ProviderName()
}

// Additional methods.

func (a *storageAdapter) UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, opts *UploadOptions) (*UploadResult, error) {
	if a.cfg.MaxFileSize > 0 && header.Size > a.cfg.MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max: %d)", header.Size, a.cfg.MaxFileSize)
	}
	// Validate MIME type if allowed list is set
	if len(a.cfg.AllowedMime) > 0 {
		contentType := header.Header.Get("Content-Type")
		// If not present, detect
		if contentType == "" {
			buf := make([]byte, 512)
			file.Read(buf)
			file.Seek(0, 0)
			contentType = http.DetectContentType(buf)
		}
		if !a.cfg.AllowedMime[contentType] {
			return nil, fmt.Errorf("file type not allowed: %s", contentType)
		}
	}
	return a.Upload(ctx, file, header.Filename, opts)
}

func (a *storageAdapter) UploadBuffer(ctx context.Context, data []byte, filename string, opts *UploadOptions) (*UploadResult, error) {
	reader := bytes.NewReader(data)
	return a.Upload(ctx, reader, filename, opts)
}

func (a *storageAdapter) Exists(ctx context.Context, filename string) (bool, error) {
	_, err := a.provider.GetMetadata(ctx, filename)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *storageAdapter) Copy(ctx context.Context, src, dst string) error {
	// For simplicity, download and re-upload.
	// Not efficient for large files; better to implement provider-specific copy.
	var buf bytes.Buffer
	if err := a.Download(ctx, src, &buf); err != nil {
		return err
	}
	_, err := a.Upload(ctx, &buf, dst, &UploadOptions{})
	return err
}

func (a *storageAdapter) Move(ctx context.Context, src, dst string) error {
	if err := a.Copy(ctx, src, dst); err != nil {
		return err
	}
	return a.Delete(ctx, src)
}

func (a *storageAdapter) GetThumbnail(ctx context.Context, filename string, size int) (string, error) {
	if !a.SupportsTransformations() {
		// Fallback: generate thumbnail locally
		var buf bytes.Buffer
		if err := a.Download(ctx, filename, &buf); err != nil {
			return "", err
		}
		img, format, err := image.Decode(&buf)
		if err != nil {
			return "", fmt.Errorf("failed to decode image: %w", err)
		}
		// Resize
		bounds := img.Bounds()
		origW, origH := bounds.Dx(), bounds.Dy()
		var newW, newH int
		if origW > origH {
			newW = size
			newH = (size * origH) / origW
		} else {
			newH = size
			newW = (size * origW) / origH
		}
		dstImg := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.NearestNeighbor.Scale(dstImg, dstImg.Bounds(), img, bounds, draw.Over, nil)
		// Encode to a buffer
		outBuf := bytes.NewBuffer(nil)
		var encodeErr error
		switch format {
		case "jpeg":
			encodeErr = jpeg.Encode(outBuf, dstImg, &jpeg.Options{Quality: 85})
		case "png":
			encodeErr = png.Encode(outBuf, dstImg)
		case "gif":
			encodeErr = gif.Encode(outBuf, dstImg, &gif.Options{NumColors: 256})
		default:
			return "", fmt.Errorf("unsupported format: %s", format)
		}
		if encodeErr != nil {
			return "", encodeErr
		}
		// Upload the thumbnail
		thumbFilename := "thumb_" + filename
		result, err := a.Upload(ctx, outBuf, thumbFilename, &UploadOptions{})
		if err != nil {
			return "", err
		}
		return result.URL, nil
	}
	// Use provider's transformations
	transform := TransformOptions{
		Width:  size,
		Height: size,
		Crop:   "fill",
	}
	return a.Transform(ctx, filename, transform)
}

func (a *storageAdapter) Close() error {
	// No-op for now; could close connections if needed.
	return nil
}

// Helper functions.

func sanitizeFilename(filename string) string {
	// Remove dangerous characters
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_. ]`)
	safe := re.ReplaceAllString(filename, "_")
	// Collapse multiple spaces
	reSpace := regexp.MustCompile(`\s+`)
	safe = reSpace.ReplaceAllString(safe, "_")
	return strings.ToLower(safe)
}

func detectContentType(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	head := make([]byte, 512)
	_, err = file.Read(head)
	if err != nil {
		return "", err
	}
	return http.DetectContentType(head), nil
}

// Global default adapter (optional)
var defaultStorageAdapter StorageAdapter
var storageOnce sync.Once

// InitStorageAdapter initializes the global storage adapter.
func InitStorageAdapter(providerName string, config map[string]string) error {
	var err error
	storageOnce.Do(func() {
		defaultStorageAdapter, err = NewStorageAdapter(providerName, config)
	})
	return err
}

// GetStorageAdapter returns the global adapter.
func GetStorageAdapter() StorageAdapter {
	if defaultStorageAdapter == nil {
		panic("storage adapter not initialized")
	}
	return defaultStorageAdapter
}