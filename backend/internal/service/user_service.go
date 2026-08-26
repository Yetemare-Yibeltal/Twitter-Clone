// backend/internal/service/user_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/domain/valueobjects"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	MaxBioLength      = 160
	MaxFullNameLength = 100
	MaxLocationLength = 100
	MaxWebsiteLength  = 200
	DefaultUserLimit  = 20
	MaxUserLimit      = 100
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrUserSuspended         = errors.New("user is suspended")
	ErrUserInactive          = errors.New("user is inactive")
	ErrUserAlreadyExists     = errors.New("user already exists")
	ErrDuplicateUsername     = errors.New("username already taken")
	ErrDuplicateEmail        = errors.New("email already registered")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrInvalidUserID         = errors.New("invalid user ID")
	ErrInvalidUsername       = errors.New("invalid username")
	ErrInvalidEmail          = errors.New("invalid email")
	ErrInvalidFullName       = errors.New("invalid full name")
	ErrBioTooLong            = errors.New("bio exceeds maximum length")
	ErrAvatarURLInvalid      = errors.New("invalid avatar URL")
	ErrBannerURLInvalid      = errors.New("invalid banner URL")
	ErrWebsiteInvalid        = errors.New("invalid website URL")
	ErrLocationTooLong       = errors.New("location is too long")
	ErrOldPasswordIncorrect  = errors.New("old password is incorrect")
	ErrNewPasswordSame       = errors.New("new password must be different from old password")
	ErrInvalidPassword       = errors.New("invalid password")
	ErrCannotDeleteSelf      = errors.New("cannot delete your own account")
)

// ======================================================================
// UserService Interface
// ======================================================================

// UserService defines the user service interface.
type UserService interface {
	// GetUserByID retrieves a user by ID.
	GetUserByID(ctx context.Context, userID string) (*entities.User, error)
	
	// GetUserByUsername retrieves a user by username.
	GetUserByUsername(ctx context.Context, username string) (*entities.User, error)
	
	// GetUserByEmail retrieves a user by email.
	GetUserByEmail(ctx context.Context, email string) (*entities.User, error)
	
	// GetProfile returns a user's profile with additional stats.
	GetProfile(ctx context.Context, identifier, currentUserID string) (*dto.UserProfileResponse, error)
	
	// UpdateProfile updates a user's profile.
	UpdateProfile(ctx context.Context, userID string, req *dto.UpdateProfileRequest) (*dto.UserProfileResponse, error)
	
	// UpdateSettings updates a user's settings.
	UpdateSettings(ctx context.Context, userID string, req *dto.UpdateSettingsRequest) (*dto.UserSettingsResponse, error)
	
	// ChangePassword changes a user's password.
	ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error
	
	// ChangeEmail changes a user's email.
	ChangeEmail(ctx context.Context, userID, newEmail string) error
	
	// GetUserStats returns user statistics.
	GetUserStats(ctx context.Context, userID string) (*dto.UserStatsResponse, error)
	
	// CheckUserExists checks if a user exists.
	CheckUserExists(ctx context.Context, userID string) (bool, error)
	
	// GetUsersByIDs returns users by their IDs.
	GetUsersByIDs(ctx context.Context, ids []string) ([]*entities.User, error)
	
	// GetFollowingStatus checks if a user follows another.
	GetFollowingStatus(ctx context.Context, followerID, followeeID string) (bool, error)
	
	// SearchUsers searches users by query.
	SearchUsers(ctx context.Context, query string, cursor string, limit int, currentUserID string) ([]*dto.UserSearchResponse, string, int64, error)
	
	// DeleteUser deletes a user account (soft delete).
	DeleteUser(ctx context.Context, userID string) error
	
	// ActivateUser activates a user account.
	ActivateUser(ctx context.Context, userID string) error
	
	// DeactivateUser deactivates a user account.
	DeactivateUser(ctx context.Context, userID string) error
	
	// SuspendUser suspends a user account.
	SuspendUser(ctx context.Context, userID, reason string) error
	
	// UnsuspendUser unsuspends a user account.
	UnsuspendUser(ctx context.Context, userID string) error
	
	// VerifyUser marks a user as verified.
	VerifyUser(ctx context.Context, userID string) error
	
	// UnverifyUser removes verified status from a user.
	UnverifyUser(ctx context.Context, userID string) error
	
	// CreateUser creates a new user (used by auth service).
	CreateUser(ctx context.Context, user *entities.User) error
	
	// GetUserCounts returns follower and following counts for a user.
	GetUserCounts(ctx context.Context, userID string) (*dto.FollowCountsResponse, error)
	
	// GetFollowers returns paginated followers list.
	GetFollowers(ctx context.Context, userID string, cursor string, limit int, currentUserID string) ([]*dto.FollowerResponse, string, int64, error)
	
	// GetFollowing returns paginated following list.
	GetFollowing(ctx context.Context, userID string, cursor string, limit int, currentUserID string) ([]*dto.FollowerResponse, string, int64, error)
}

// ======================================================================
// userService Implementation
// ======================================================================

// userService implements UserService.
type userService struct {
	userRepo     interfaces.UserRepository
	followRepo   interfaces.FollowRepository
	tweetRepo    interfaces.TweetRepository
	likeRepo     interfaces.LikeRepository
	redisAdapter adapter.RedisAdapter
	log          *logrus.Entry
}

// NewUserService creates a new user service.
func NewUserService(
	userRepo interfaces.UserRepository,
	followRepo interfaces.FollowRepository,
	tweetRepo interfaces.TweetRepository,
	likeRepo interfaces.LikeRepository,
	redisAdapter adapter.RedisAdapter,
) UserService {
	return &userService{
		userRepo:     userRepo,
		followRepo:   followRepo,
		tweetRepo:    tweetRepo,
		likeRepo:     likeRepo,
		redisAdapter: redisAdapter,
		log:          logger.WithField("service", "user"),
	}
}

// ======================================================================
// Get User by ID
// ======================================================================

// GetUserByID retrieves a user by ID.
func (s *userService) GetUserByID(ctx context.Context, userID string) (*entities.User, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("user:%s", userID)
	if s.redisAdapter != nil {
		var user entities.User
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &user); err == nil {
			return &user, nil
		}
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Cache for 5 minutes
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, user, 5*time.Minute)
	}
	return user, nil
}

// ======================================================================
// Get User by Username
// ======================================================================

// GetUserByUsername retrieves a user by username.
func (s *userService) GetUserByUsername(ctx context.Context, username string) (*entities.User, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}
	return user, nil
}

// ======================================================================
// Get User by Email
// ======================================================================

// GetUserByEmail retrieves a user by email.
func (s *userService) GetUserByEmail(ctx context.Context, email string) (*entities.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return user, nil
}

// ======================================================================
// Get Profile
// ======================================================================

// GetProfile returns a user's profile with additional stats.
func (s *userService) GetProfile(ctx context.Context, identifier, currentUserID string) (*dto.UserProfileResponse, error) {
	// Try to get user by ID or username
	var user *entities.User
	var err error
	// Check if identifier is a UUID (ID) or username
	if strings.Contains(identifier, "-") && len(identifier) == 36 {
		user, err = s.GetUserByID(ctx, identifier)
	} else {
		user, err = s.GetUserByUsername(ctx, identifier)
	}
	if err != nil {
		return nil, err
	}
	// Check user status
	if user.IsSuspended() {
		return nil, ErrUserSuspended
	}
	if user.IsInactive() {
		return nil, ErrUserInactive
	}
	// Get counts
	followers, err := s.followRepo.CountFollowers(ctx, user.ID)
	if err != nil {
		followers = 0
	}
	following, err := s.followRepo.CountFollowing(ctx, user.ID)
	if err != nil {
		following = 0
	}
	tweetCount, err := s.tweetRepo.CountByUserID(ctx, user.ID)
	if err != nil {
		tweetCount = 0
	}
	// Check follow status
	isFollowing := false
	isMutual := false
	if currentUserID != "" && currentUserID != user.ID {
		isFollowing, err = s.followRepo.Exists(ctx, currentUserID, user.ID)
		if err == nil && isFollowing {
			isMutual, _ = s.followRepo.AreMutual(ctx, currentUserID, user.ID)
		}
	}
	// Get recent tweets (last 5)
	recentTweets, _, err := s.tweetRepo.GetByUserID(ctx, user.ID, "", 5, false)
	if err != nil {
		recentTweets = []*entities.Tweet{}
	}
	// Build response
	response := &dto.UserProfileResponse{
		ID:           user.ID,
		Username:     user.Username,
		Email:        user.Email,
		FullName:     user.FullName,
		Bio:          user.Bio,
		AvatarURL:    user.AvatarURL,
		BannerURL:    user.BannerURL,
		IsVerified:   user.IsVerified,
		IsSuspended:  user.IsSuspended(),
		IsActive:     user.IsActive(),
		Role:         user.Role,
		Followers:    followers,
		Following:    following,
		TweetCount:   tweetCount,
		IsFollowing:  isFollowing,
		IsMutual:     isMutual,
		JoinedAt:     user.CreatedAt,
		LastActive:   user.LastActiveAt,
		RecentTweets: recentTweets,
	}
	return response, nil
}

// ======================================================================
// Update Profile
// ======================================================================

// UpdateProfile updates a user's profile.
func (s *userService) UpdateProfile(ctx context.Context, userID string, req *dto.UpdateProfileRequest) (*dto.UserProfileResponse, error) {
	// Get user
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Check status
	if user.IsSuspended() {
		return nil, ErrUserSuspended
	}
	if user.IsInactive() {
		return nil, ErrUserInactive
	}
	// Validate and update fields
	if req.FullName != "" {
		if len(req.FullName) > MaxFullNameLength {
			return nil, ErrInvalidFullName
		}
		user.FullName = req.FullName
	}
	if req.Bio != "" {
		if len(req.Bio) > MaxBioLength {
			return nil, ErrBioTooLong
		}
		user.Bio = req.Bio
	}
	if req.Location != "" {
		if len(req.Location) > MaxLocationLength {
			return nil, ErrLocationTooLong
		}
		user.Location = req.Location
	}
	if req.Website != "" {
		if len(req.Website) > MaxWebsiteLength {
			return nil, ErrWebsiteInvalid
		}
		if !isValidURL(req.Website) {
			return nil, ErrWebsiteInvalid
		}
		user.Website = req.Website
	}
	if req.AvatarURL != nil {
		if *req.AvatarURL != "" && !isValidURL(*req.AvatarURL) {
			return nil, ErrAvatarURLInvalid
		}
		user.AvatarURL = *req.AvatarURL
	}
	if req.BannerURL != nil {
		if *req.BannerURL != "" && !isValidURL(*req.BannerURL) {
			return nil, ErrBannerURLInvalid
		}
		user.BannerURL = *req.BannerURL
	}
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateUserCache(ctx, user.ID)
	// Get updated profile
	return s.GetProfile(ctx, user.ID, user.ID)
}

// ======================================================================
// Update Settings
// ======================================================================

// UpdateSettings updates a user's settings.
func (s *userService) UpdateSettings(ctx context.Context, userID string, req *dto.UpdateSettingsRequest) (*dto.UserSettingsResponse, error) {
	// Get user
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.IsSuspended() {
		return nil, ErrUserSuspended
	}
	if user.IsInactive() {
		return nil, ErrUserInactive
	}
	// Update metadata
	if req.Theme != "" {
		user.Metadata.Theme = req.Theme
	}
	if req.Language != "" {
		user.Metadata.Language = req.Language
	}
	if req.Timezone != "" {
		user.Metadata.Timezone = req.Timezone
	}
	if req.Notifications != nil {
		if user.Metadata.Notifications == nil {
			user.Metadata.Notifications = make(map[string]bool)
		}
		for k, v := range req.Notifications {
			user.Metadata.Notifications[k] = v
		}
	}
	if req.Privacy != nil {
		if user.Metadata.Privacy == nil {
			user.Metadata.Privacy = make(map[string]bool)
		}
		for k, v := range req.Privacy {
			user.Metadata.Privacy[k] = v
		}
	}
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update settings: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateUserCache(ctx, user.ID)
	return &dto.UserSettingsResponse{
		Theme:        user.Metadata.Theme,
		Language:     user.Metadata.Language,
		Timezone:     user.Metadata.Timezone,
		Notifications: user.Metadata.Notifications,
		Privacy:      user.Metadata.Privacy,
		UpdatedAt:    user.UpdatedAt,
	}, nil
}

// ======================================================================
// Change Password
// ======================================================================

// ChangePassword changes a user's password.
func (s *userService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsSuspended() {
		return ErrUserSuspended
	}
	if user.IsInactive() {
		return ErrUserInactive
	}
	// Verify old password
	if !user.CheckPassword(oldPassword) {
		return ErrOldPasswordIncorrect
	}
	// Check if new password is same as old
	if oldPassword == newPassword {
		return ErrNewPasswordSame
	}
	// Change password
	if err := user.SetPassword(newPassword); err != nil {
		return fmt.Errorf("failed to change password: %w", err)
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to save new password: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateUserCache(ctx, user.ID)
	s.log.WithField("user_id", userID).Info("Password changed")
	return nil
}

// ======================================================================
// Change Email
// ======================================================================

// ChangeEmail changes a user's email.
func (s *userService) ChangeEmail(ctx context.Context, userID, newEmail string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsSuspended() {
		return ErrUserSuspended
	}
	if user.IsInactive() {
		return ErrUserInactive
	}
	// Validate email
	_, err = valueobjects.NewEmail(newEmail)
	if err != nil {
		return ErrInvalidEmail
	}
	// Check if email is taken
	exists, err := s.userRepo.ExistsByEmail(ctx, newEmail)
	if err != nil {
		return fmt.Errorf("failed to check email: %w", err)
	}
	if exists && newEmail != user.Email {
		return ErrDuplicateEmail
	}
	user.Email = newEmail
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update email: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateUserCache(ctx, user.ID)
	s.log.WithField("user_id", userID).Info("Email changed")
	return nil
}

// ======================================================================
// Get User Stats
// ======================================================================

// GetUserStats returns user statistics.
func (s *userService) GetUserStats(ctx context.Context, userID string) (*dto.UserStatsResponse, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Get counts
	followers, err := s.followRepo.CountFollowers(ctx, userID)
	if err != nil {
		followers = 0
	}
	following, err := s.followRepo.CountFollowing(ctx, userID)
	if err != nil {
		following = 0
	}
	tweetCount, err := s.tweetRepo.CountByUserID(ctx, userID)
	if err != nil {
		tweetCount = 0
	}
	likeCount, err := s.likeRepo.CountByUserID(ctx, userID)
	if err != nil {
		likeCount = 0
	}
	return &dto.UserStatsResponse{
		UserID:      userID,
		Username:    user.Username,
		FullName:    user.FullName,
		Followers:   followers,
		Following:   following,
		TweetCount:  tweetCount,
		LikeCount:   likeCount,
		JoinedAt:    user.CreatedAt,
		LastActive:  user.LastActiveAt,
		IsVerified:  user.IsVerified,
		IsSuspended: user.IsSuspended(),
	}, nil
}

// ======================================================================
= Check User Exists
// ======================================================================

// CheckUserExists checks if a user exists.
func (s *userService) CheckUserExists(ctx context.Context, userID string) (bool, error) {
	exists, err := s.userRepo.Exists(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}
	return exists, nil
}

// ======================================================================
= Get Users by IDs
// ======================================================================

// GetUsersByIDs returns users by their IDs.
func (s *userService) GetUsersByIDs(ctx context.Context, ids []string) ([]*entities.User, error) {
	if len(ids) == 0 {
		return []*entities.User{}, nil
	}
	users, err := s.userRepo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	return users, nil
}

// ======================================================================
= Get Following Status
// ======================================================================

// GetFollowingStatus checks if a user follows another.
func (s *userService) GetFollowingStatus(ctx context.Context, followerID, followeeID string) (bool, error) {
	if followerID == followeeID {
		return false, nil
	}
	exists, err := s.followRepo.Exists(ctx, followerID, followeeID)
	if err != nil {
		return false, fmt.Errorf("failed to check following status: %w", err)
	}
	return exists, nil
}

// ======================================================================
= Search Users
// ======================================================================

// SearchUsers searches users by query.
func (s *userService) SearchUsers(ctx context.Context, query string, cursor string, limit int, currentUserID string) ([]*dto.UserSearchResponse, string, int64, error) {
	if limit < 1 || limit > MaxUserLimit {
		limit = DefaultUserLimit
	}
	// Search users from repository
	users, total, err := s.userRepo.Search(ctx, query, nil)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to search users: %w", err)
	}
	responses := make([]*dto.UserSearchResponse, 0, len(users))
	for _, user := range users {
		// Check if current user follows this user
		isFollowing := false
		if currentUserID != "" && currentUserID != user.ID {
			isFollowing, _ = s.followRepo.Exists(ctx, currentUserID, user.ID)
		}
		// Check if mutual
		isMutual := false
		if currentUserID != "" && isFollowing {
			mutual, _ := s.followRepo.AreMutual(ctx, currentUserID, user.ID)
			isMutual = mutual
		}
		responses = append(responses, &dto.UserSearchResponse{
			ID:            user.ID,
			Username:      user.Username,
			FullName:      user.FullName,
			AvatarURL:     user.AvatarURL,
			Bio:           user.Bio,
			IsVerified:    user.IsVerified,
			IsFollowing:   isFollowing,
			IsMutual:      isMutual,
			FollowerCount: user.FollowerCount,
			TweetCount:    user.TweetCount,
		})
	}
	return responses, "", int64(len(responses)), nil
}

// ======================================================================
= Create User
// ======================================================================

// CreateUser creates a new user (used by auth service).
func (s *userService) CreateUser(ctx context.Context, user *entities.User) error {
	// Check if username exists
	exists, err := s.userRepo.ExistsByUsername(ctx, user.Username)
	if err != nil {
		return fmt.Errorf("failed to check username: %w", err)
	}
	if exists {
		return ErrDuplicateUsername
	}
	// Check if email exists
	exists, err = s.userRepo.ExistsByEmail(ctx, user.Email)
	if err != nil {
		return fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return ErrDuplicateEmail
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// ======================================================================
= Delete User
// ======================================================================

// DeleteUser deletes a user account (soft delete).
func (s *userService) DeleteUser(ctx context.Context, userID string) error {
	// Check if user exists
	_, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	// Soft delete user
	if err := s.userRepo.SoftDelete(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateUserCache(ctx, userID)
	s.log.WithField("user_id", userID).Info("User deleted")
	return nil
}

// ======================================================================
= Activate/Deactivate User
// ======================================================================

// ActivateUser activates a user account.
func (s *userService) ActivateUser(ctx context.Context, userID string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := user.Activate(); err != nil {
		return err
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to activate user: %w", err)
	}
	_ = s.invalidateUserCache(ctx, userID)
	return nil
}

// DeactivateUser deactivates a user account.
func (s *userService) DeactivateUser(ctx context.Context, userID string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := user.Deactivate(); err != nil {
		return err
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to deactivate user: %w", err)
	}
	_ = s.invalidateUserCache(ctx, userID)
	return nil
}

// ======================================================================
= Suspend/Unsuspend User
// ======================================================================

// SuspendUser suspends a user account.
func (s *userService) SuspendUser(ctx context.Context, userID, reason string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := user.Suspend(reason); err != nil {
		return err
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to suspend user: %w", err)
	}
	_ = s.invalidateUserCache(ctx, userID)
	s.log.WithFields(logrus.Fields{
		"user_id": userID,
		"reason":  reason,
	}).Info("User suspended")
	return nil
}

// UnsuspendUser unsuspends a user account.
func (s *userService) UnsuspendUser(ctx context.Context, userID string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := user.Unsuspend(); err != nil {
		return err
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to unsuspend user: %w", err)
	}
	_ = s.invalidateUserCache(ctx, userID)
	s.log.WithField("user_id", userID).Info("User unsuspended")
	return nil
}

// ======================================================================
= Verify/Unverify User
// ======================================================================

// VerifyUser marks a user as verified.
func (s *userService) VerifyUser(ctx context.Context, userID string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := user.Verify(); err != nil {
		return err
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to verify user: %w", err)
	}
	_ = s.invalidateUserCache(ctx, userID)
	return nil
}

// UnverifyUser removes verified status from a user.
func (s *userService) UnverifyUser(ctx context.Context, userID string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := user.Unverify(); err != nil {
		return err
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to unverify user: %w", err)
	}
	_ = s.invalidateUserCache(ctx, userID)
	return nil
}

// ======================================================================
= Get User Counts
// ======================================================================

// GetUserCounts returns follower and following counts for a user.
func (s *userService) GetUserCounts(ctx context.Context, userID string) (*dto.FollowCountsResponse, error) {
	// Check if user exists
	_, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	followers, err := s.followRepo.CountFollowers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count followers: %w", err)
	}
	following, err := s.followRepo.CountFollowing(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count following: %w", err)
	}
	return &dto.FollowCountsResponse{
		UserID:    userID,
		Followers: followers,
		Following: following,
	}, nil
}

// ======================================================================
= Get Followers
// ======================================================================

// GetFollowers returns paginated followers list.
func (s *userService) GetFollowers(ctx context.Context, userID string, cursor string, limit int, currentUserID string) ([]*dto.FollowerResponse, string, int64, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Check if user exists
	_, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, "", 0, err
	}
	follows, nextCursor, err := s.followRepo.GetFollowers(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get followers: %w", err)
	}
	total, _ := s.followRepo.CountFollowers(ctx, userID)
	responses := make([]*dto.FollowerResponse, 0, len(follows))
	for _, f := range follows {
		user, err := s.GetUserByID(ctx, f.FollowerID)
		if err != nil {
			continue
		}
		isFollowing := false
		if currentUserID != "" && currentUserID != userID {
			isFollowing, _ = s.followRepo.Exists(ctx, currentUserID, f.FollowerID)
		}
		isMutual := false
		if currentUserID != "" {
			mutual, _ := s.followRepo.AreMutual(ctx, currentUserID, f.FollowerID)
			isMutual = mutual
		}
		responses = append(responses, &dto.FollowerResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			IsFollowing: isFollowing,
			IsMutual:    isMutual,
			FollowedAt:  f.CreatedAt,
		})
	}
	return responses, nextCursor, total, nil
}

// ======================================================================
= Get Following
// ======================================================================

// GetFollowing returns paginated following list.
func (s *userService) GetFollowing(ctx context.Context, userID string, cursor string, limit int, currentUserID string) ([]*dto.FollowerResponse, string, int64, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Check if user exists
	_, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, "", 0, err
	}
	follows, nextCursor, err := s.followRepo.GetFollowing(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get following: %w", err)
	}
	total, _ := s.followRepo.CountFollowing(ctx, userID)
	responses := make([]*dto.FollowerResponse, 0, len(follows))
	for _, f := range follows {
		user, err := s.GetUserByID(ctx, f.FolloweeID)
		if err != nil {
			continue
		}
		isFollowing := false
		if currentUserID != "" && currentUserID != userID {
			isFollowing, _ = s.followRepo.Exists(ctx, currentUserID, f.FolloweeID)
		}
		isMutual := false
		if currentUserID != "" {
			mutual, _ := s.followRepo.AreMutual(ctx, currentUserID, f.FolloweeID)
			isMutual = mutual
		}
		responses = append(responses, &dto.FollowerResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			IsFollowing: isFollowing,
			IsMutual:    isMutual,
			FollowedAt:  f.CreatedAt,
		})
	}
	return responses, nextCursor, total, nil
}

// ======================================================================
= Cache Invalidation
// ======================================================================

// invalidateUserCache invalidates user cache.
func (s *userService) invalidateUserCache(ctx context.Context, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	cacheKey := fmt.Sprintf("user:%s", userID)
	return s.redisAdapter.Delete(ctx, cacheKey)
}

// ======================================================================
= Helper Functions
// ======================================================================

// isValidURL checks if a URL is valid.
func isValidURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// ======================================================================
= Global Instance
// ======================================================================

var defaultUserService UserService

// InitUserService initializes the global user service.
func InitUserService(
	userRepo interfaces.UserRepository,
	followRepo interfaces.FollowRepository,
	tweetRepo interfaces.TweetRepository,
	likeRepo interfaces.LikeRepository,
	redisAdapter adapter.RedisAdapter,
) {
	defaultUserService = NewUserService(
		userRepo,
		followRepo,
		tweetRepo,
		likeRepo,
		redisAdapter,
	)
}

// GetUserService returns the global user service.
func GetUserService() UserService {
	if defaultUserService == nil {
		panic("user service not initialized")
	}
	return defaultUserService
}