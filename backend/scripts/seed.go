// backend/scripts/seed.go
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// ======================================================================
// Constants
// ======================================================================

const (
	DefaultUsers    = 10
	DefaultTweets   = 50
	DefaultFollows  = 30
	DefaultLikes    = 40
	DefaultRetweets = 20
	DefaultMessages = 30
	DefaultBookmarks = 15
	DefaultPolls    = 5
	DefaultCommunities = 3
	DefaultReports  = 5
)

var (
	dbURL     = flag.String("db", "", "Database connection URL")
	users     = flag.Int("users", DefaultUsers, "Number of users to seed")
	tweets    = flag.Int("tweets", DefaultTweets, "Number of tweets to seed")
	follows   = flag.Int("follows", DefaultFollows, "Number of follows to seed")
	likes     = flag.Int("likes", DefaultLikes, "Number of likes to seed")
	retweets  = flag.Int("retweets", DefaultRetweets, "Number of retweets to seed")
	messages  = flag.Int("messages", DefaultMessages, "Number of messages to seed")
	bookmarks = flag.Int("bookmarks", DefaultBookmarks, "Number of bookmarks to seed")
	polls     = flag.Int("polls", DefaultPolls, "Number of polls to seed")
	communities = flag.Int("communities", DefaultCommunities, "Number of communities to seed")
	reports   = flag.Int("reports", DefaultReports, "Number of reports to seed")
	clean     = flag.Bool("clean", false, "Clean existing data before seeding")
	verbose   = flag.Bool("verbose", false, "Verbose output")
)

// ======================================================================
= Types
// ======================================================================

type User struct {
	ID         string
	Username   string
	Email      string
	Password   string
	FullName   string
	Bio        string
	AvatarURL  string
	CreatedAt  time.Time
}

type Tweet struct {
	ID         string
	UserID     string
	Content    string
	MediaURLs  []string
	ParentID   *string
	RetweetOfID *string
	IsPoll     bool
	CreatedAt  time.Time
}

type Follow struct {
	FollowerID string
	FolloweeID string
	CreatedAt  time.Time
}

type Like struct {
	TweetID   string
	UserID    string
	CreatedAt time.Time
}

type Retweet struct {
	TweetID   string
	UserID    string
	CreatedAt time.Time
}

type Bookmark struct {
	TweetID   string
	UserID    string
	CreatedAt time.Time
}

type Message struct {
	ID         string
	SenderID   string
	ReceiverID string
	Content    string
	Read       bool
	CreatedAt  time.Time
}

type Notification struct {
	ID          string
	UserID      string
	FromUserID  string
	Type        string
	ReferenceID string
	Read        bool
	CreatedAt   time.Time
}

type Poll struct {
	ID        string
	TweetID   string
	Options   []string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Community struct {
	ID          string
	Name        string
	Slug        string
	Description string
	CreatedBy   string
	IsPrivate   bool
	CreatedAt   time.Time
}

type CommunityMember struct {
	CommunityID string
	UserID      string
	Role        string
	JoinedAt    time.Time
}

type Report struct {
	ID          string
	ReporterID  string
	TargetID    string
	TargetType  string
	Reason      string
	Status      string
	Severity    string
	CreatedAt   time.Time
}

// ======================================================================
= Main
// ======================================================================

func main() {
	flag.Parse()

	if *dbURL == "" {
		*dbURL = os.Getenv("DATABASE_URL")
		if *dbURL == "" {
			log.Fatal("Database URL is required. Set -db or DATABASE_URL environment variable")
		}
	}

	db, err := sql.Open("postgres", *dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Check if tables exist
	if *clean {
		if *verbose {
			log.Println("Cleaning existing data...")
		}
		if err := cleanDatabase(db); err != nil {
			log.Fatalf("Failed to clean database: %v", err)
		}
	}

	seed := NewSeeder(db)
	if err := seed.Run(); err != nil {
		log.Fatalf("Failed to seed database: %v", err)
	}

	log.Println("Database seeded successfully!")
}

// ======================================================================
= Seeder
// ======================================================================

type Seeder struct {
	db        *sql.DB
	users     []*User
	tweets    []*Tweet
	communities []*Community
	rand      *rand.Rand
}

func NewSeeder(db *sql.DB) *Seeder {
	return &Seeder{
		db:   db,
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *Seeder) Run() error {
	// Seed users first
	if err := s.seedUsers(); err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	// Seed communities
	if err := s.seedCommunities(); err != nil {
		return fmt.Errorf("failed to seed communities: %w", err)
	}

	// Seed tweets
	if err := s.seedTweets(); err != nil {
		return fmt.Errorf("failed to seed tweets: %w", err)
	}

	// Seed follows
	if err := s.seedFollows(); err != nil {
		return fmt.Errorf("failed to seed follows: %w", err)
	}

	// Seed likes
	if err := s.seedLikes(); err != nil {
		return fmt.Errorf("failed to seed likes: %w", err)
	}

	// Seed retweets
	if err := s.seedRetweets(); err != nil {
		return fmt.Errorf("failed to seed retweets: %w", err)
	}

	// Seed bookmarks
	if err := s.seedBookmarks(); err != nil {
		return fmt.Errorf("failed to seed bookmarks: %w", err)
	}

	// Seed messages
	if err := s.seedMessages(); err != nil {
		return fmt.Errorf("failed to seed messages: %w", err)
	}

	// Seed notifications
	if err := s.seedNotifications(); err != nil {
		return fmt.Errorf("failed to seed notifications: %w", err)
	}

	// Seed polls
	if err := s.seedPolls(); err != nil {
		return fmt.Errorf("failed to seed polls: %w", err)
	}

	// Seed reports
	if err := s.seedReports(); err != nil {
		return fmt.Errorf("failed to seed reports: %w", err)
	}

	return nil
}

// ======================================================================
= Seed Users
// ======================================================================

func (s *Seeder) seedUsers() error {
	log.Printf("Seeding %d users...", *users)

	// Default users
	defaultUsers := []struct {
		username string
		email    string
		fullName string
		bio      string
	}{
		{"admin", "admin@example.com", "Admin User", "System administrator"},
		{"john_doe", "john@example.com", "John Doe", "Software engineer and tech enthusiast"},
		{"jane_smith", "jane@example.com", "Jane Smith", "Product designer and UX advocate"},
		{"bob_wilson", "bob@example.com", "Bob Wilson", "Full-stack developer"},
		{"alice_brown", "alice@example.com", "Alice Brown", "Data scientist and AI researcher"},
		{"charlie_davis", "charlie@example.com", "Charlie Davis", "DevOps engineer"},
		{"diana_miller", "diana@example.com", "Diana Miller", "Frontend specialist"},
		{"edward_jones", "edward@example.com", "Edward Jones", "Backend engineer"},
		{"fiona_clark", "fiona@example.com", "Fiona Clark", "Mobile developer"},
		{"george_white", "george@example.com", "George White", "Cloud architect"},
		{"hannah_lee", "hannah@example.com", "Hannah Lee", "Security engineer"},
		{"ian_king", "ian@example.com", "Ian King", "Technical writer"},
	}

	// Create users
	for _, u := range defaultUsers {
		if len(s.users) >= *users {
			break
		}
		user := &User{
			ID:        uuid.New().String(),
			Username:  u.username,
			Email:     u.email,
			Password:  "password123",
			FullName:  u.fullName,
			Bio:       u.bio,
			AvatarURL: fmt.Sprintf("https://ui-avatars.com/api/?name=%s&size=200", strings.ReplaceAll(u.fullName, " ", "+")),
			CreatedAt: randomPastTime(30),
		}
		if err := s.insertUser(user); err != nil {
			return err
		}
		s.users = append(s.users, user)
	}

	// Create additional random users
	for len(s.users) < *users {
		username := fmt.Sprintf("user_%d", len(s.users)+1)
		user := &User{
			ID:        uuid.New().String(),
			Username:  username,
			Email:     fmt.Sprintf("%s@example.com", username),
			Password:  "password123",
			FullName:  fmt.Sprintf("User %d", len(s.users)+1),
			Bio:       randomBio(),
			AvatarURL: fmt.Sprintf("https://ui-avatars.com/api/?name=User+%d&size=200", len(s.users)+1),
			CreatedAt: randomPastTime(30),
		}
		if err := s.insertUser(user); err != nil {
			return err
		}
		s.users = append(s.users, user)
	}

	log.Printf("Seeded %d users", len(s.users))
	return nil
}

func (s *Seeder) insertUser(user *User) error {
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO users (id, username, email, password_hash, full_name, bio, avatar_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = s.db.Exec(query,
		user.ID, user.Username, user.Email, string(hash),
		user.FullName, user.Bio, user.AvatarURL, user.CreatedAt,
	)
	return err
}

// ======================================================================
= Seed Communities
// ======================================================================

func (s *Seeder) seedCommunities() error {
	if *communities <= 0 {
		return nil
	}
	log.Printf("Seeding %d communities...", *communities)

	if len(s.users) < 2 {
		return nil
	}

	communityData := []struct {
		name        string
		slug        string
		description string
		isPrivate   bool
	}{
		{"Golang Developers", "golang-dev", "Community for Go programming language enthusiasts", false},
		{"Tech News", "tech-news", "Latest technology news and discussions", false},
		{"Open Source", "open-source", "Collaborate on open source projects", false},
		{"Web Developers", "web-dev", "Modern web development community", false},
		{"AI & Machine Learning", "ai-ml", "Artificial intelligence and machine learning", true},
		{"Cybersecurity", "cybersec", "Security research and best practices", false},
	}

	for i, cd := range communityData {
		if i >= *communities {
			break
		}
		createdBy := s.users[i%len(s.users)].ID
		community := &Community{
			ID:          uuid.New().String(),
			Name:        cd.name,
			Slug:        cd.slug,
			Description: cd.description,
			CreatedBy:   createdBy,
			IsPrivate:   cd.isPrivate,
			CreatedAt:   randomPastTime(20),
		}
		if err := s.insertCommunity(community); err != nil {
			return err
		}
		s.communities = append(s.communities, community)

		// Add creator as owner
		member := &CommunityMember{
			CommunityID: community.ID,
			UserID:      createdBy,
			Role:        "owner",
			JoinedAt:    community.CreatedAt,
		}
		if err := s.insertCommunityMember(member); err != nil {
			return err
		}

		// Add additional members
		memberCount := 3 + s.rand.Intn(5)
		for j := 0; j < memberCount && j < len(s.users); j++ {
			if s.users[j].ID == createdBy {
				continue
			}
			role := "member"
			if j < 2 {
				role = "admin"
			}
			member := &CommunityMember{
				CommunityID: community.ID,
				UserID:      s.users[j].ID,
				Role:        role,
				JoinedAt:    randomPastTime(10),
			}
			if err := s.insertCommunityMember(member); err != nil {
				continue
			}
		}
	}

	log.Printf("Seeded %d communities", len(s.communities))
	return nil
}

func (s *Seeder) insertCommunity(community *Community) error {
	query := `
		INSERT INTO communities (id, name, slug, description, created_by, is_private, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.Exec(query,
		community.ID, community.Name, community.Slug,
		community.Description, community.CreatedBy,
		community.IsPrivate, community.CreatedAt,
	)
	return err
}

func (s *Seeder) insertCommunityMember(member *CommunityMember) error {
	query := `
		INSERT INTO community_members (community_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := s.db.Exec(query,
		member.CommunityID, member.UserID, member.Role, member.JoinedAt,
	)
	return err
}

// ======================================================================
= Seed Tweets
// ======================================================================

func (s *Seeder) seedTweets() error {
	if *tweets <= 0 {
		return nil
	}
	log.Printf("Seeding %d tweets...", *tweets)

	if len(s.users) == 0 {
		return nil
	}

	tweetContent := []string{
		"Just launched my new project! 🚀 #coding #development",
		"Excited to share my latest blog post about Go concurrency patterns. Check it out! 📝",
		"Working on something amazing... stay tuned! 🔥",
		"Just deployed to production. Fingers crossed! 🤞 #devops",
		"Learned something new today about PostgreSQL indexing. Mind blown! 🤯",
		"Contributing to open source is incredibly rewarding. #opensource #community",
		"Security is not a feature, it's a necessity. #cybersecurity #infosec",
		"AI is transforming the way we build software. #ai #machinelearning",
		"Just finished reading an amazing book on software architecture. Highly recommended! 📚",
		"Testing in production? Sometimes it's necessary, but please be careful! 😅",
		"Microservices vs Monoliths - the debate continues... #softwarearchitecture",
		"Cloud computing is the future. Embrace it or get left behind! ☁️ #cloud",
		"Writing clean code is an art form. #cleancode #programming",
		"Just fixed a bug that's been haunting me for days. Victory! 🎉 #debugging",
		"Accessibility in web development matters. Make your apps inclusive! #a11y",
		"DevOps is not just about tools, it's a culture shift. #devops #culture",
		"Rust is gaining popularity. Have you tried it yet? #rustlang #programming",
		"Data is the new oil. Process it wisely! #data #analytics",
		"Containerization changed my life. #docker #kubernetes",
		"Women in tech: we need more diversity in our industry! #womenintech #diversity",
		"Just built a new CI/CD pipeline using GitHub Actions. Game changer! #githubactions #cicd",
		"Documentation is love. Documentation is life. #documentation #techwriting",
		"Pair programming is underrated. Try it with a colleague! 👥 #pairprogramming",
		"Code reviews are essential for quality. Be kind and constructive! #codereview",
		"Technical debt is real. Pay it down regularly! #technicaldebt",
		"Just wrote my first WASM module. Mind blown! 🤯 #webassembly",
		"GraphQL vs REST - which one do you prefer? #graphql #api",
		"Serverless architecture is changing the game. #serverless",
		"Just discovered a new IDE plugin that boosts productivity 10x! 💪 #codingtools",
		"Community is everything. Grateful for this amazing dev community! ❤️",
		"Released v2.0 of my open source project! 🎉 Thanks to all contributors! #opensource",
	}

	for i := 0; i < *tweets; i++ {
		user := s.users[s.rand.Intn(len(s.users))]
		content := tweetContent[s.rand.Intn(len(tweetContent))]

		tweet := &Tweet{
			ID:        uuid.New().String(),
			UserID:    user.ID,
			Content:   content,
			MediaURLs: []string{},
			CreatedAt: randomPastTime(15),
		}

		// Add media to some tweets
		if s.rand.Intn(3) == 0 {
			mediaTypes := []string{"image/jpeg", "image/png", "image/gif", "video/mp4"}
			mediaType := mediaTypes[s.rand.Intn(len(mediaTypes))]
			tweet.MediaURLs = []string{
				fmt.Sprintf("https://example.com/media/%d.%s", s.rand.Intn(1000), strings.Split(mediaType, "/")[1]),
			}
			if s.rand.Intn(2) == 0 && len(tweet.MediaURLs) < 4 {
				tweet.MediaURLs = append(tweet.MediaURLs,
					fmt.Sprintf("https://example.com/media/%d.%s", s.rand.Intn(1000), strings.Split(mediaType, "/")[1]))
			}
		}

		// Some tweets are replies
		if s.rand.Intn(8) == 0 && len(s.tweets) > 0 {
			parent := s.tweets[s.rand.Intn(len(s.tweets))]
			tweet.ParentID = &parent.ID
			tweet.Content = fmt.Sprintf("Replying to @%s: %s", s.getUserByID(parent.UserID).Username, content)
		}

		// Some tweets are retweets (quotes)
		if s.rand.Intn(10) == 0 && len(s.tweets) > 0 {
			source := s.tweets[s.rand.Intn(len(s.tweets))]
			tweet.RetweetOfID = &source.ID
			tweet.Content = content
		}

		if err := s.insertTweet(tweet); err != nil {
			return err
		}
		s.tweets = append(s.tweets, tweet)
	}

	log.Printf("Seeded %d tweets", len(s.tweets))
	return nil
}

func (s *Seeder) insertTweet(tweet *Tweet) error {
	query := `
		INSERT INTO tweets (id, user_id, content, media_urls, parent_tweet_id, retweet_of_id, is_poll, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := s.db.Exec(query,
		tweet.ID, tweet.UserID, tweet.Content,
		pq.Array(tweet.MediaURLs),
		tweet.ParentID, tweet.RetweetOfID,
		tweet.IsPoll, tweet.CreatedAt,
	)
	return err
}

func (s *Seeder) getUserByID(id string) *User {
	for _, u := range s.users {
		if u.ID == id {
			return u
		}
	}
	return nil
}

// ======================================================================
= Seed Follows
// ======================================================================

func (s *Seeder) seedFollows() error {
	if *follows <= 0 {
		return nil
	}
	log.Printf("Seeding %d follows...", *follows)

	if len(s.users) < 2 {
		return nil
	}

	followedCount := 0
	for i := 0; i < len(s.users) && followedCount < *follows; i++ {
		follower := s.users[i]
		// Each user follows some random users
		numFollows := 1 + s.rand.Intn(3)
		for j := 0; j < numFollows && followedCount < *follows; j++ {
			followeeIdx := s.rand.Intn(len(s.users))
			if followeeIdx == i {
				continue
			}
			followee := s.users[followeeIdx]

			// Check if already following (in memory)
			if s.hasFollow(follower.ID, followee.ID) {
				continue
			}

			follow := &Follow{
				FollowerID: follower.ID,
				FolloweeID: followee.ID,
				CreatedAt:  randomPastTime(10),
			}
			if err := s.insertFollow(follow); err != nil {
				continue
			}
			followedCount++
		}
	}

	log.Printf("Seeded %d follows", followedCount)
	return nil
}

func (s *Seeder) hasFollow(followerID, followeeID string) bool {
	// Check in database
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)`
	_ = s.db.QueryRow(query, followerID, followeeID).Scan(&exists)
	return exists
}

func (s *Seeder) insertFollow(follow *Follow) error {
	query := `INSERT INTO follows (follower_id, followee_id, created_at) VALUES ($1, $2, $3)`
	_, err := s.db.Exec(query, follow.FollowerID, follow.FolloweeID, follow.CreatedAt)
	return err
}

// ======================================================================
= Seed Likes
// ======================================================================

func (s *Seeder) seedLikes() error {
	if *likes <= 0 || len(s.tweets) == 0 {
		return nil
	}
	log.Printf("Seeding %d likes...", *likes)

	likedCount := 0
	for i := 0; i < *likes; i++ {
		tweet := s.tweets[s.rand.Intn(len(s.tweets))]
		user := s.users[s.rand.Intn(len(s.users))]

		// Check if already liked
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM likes WHERE tweet_id = $1 AND user_id = $2)`
		_ = s.db.QueryRow(query, tweet.ID, user.ID).Scan(&exists)
		if exists {
			continue
		}

		like := &Like{
			TweetID:   tweet.ID,
			UserID:    user.ID,
			CreatedAt: randomPastTime(5),
		}
		if err := s.insertLike(like); err != nil {
			continue
		}
		likedCount++
	}

	log.Printf("Seeded %d likes", likedCount)
	return nil
}

func (s *Seeder) insertLike(like *Like) error {
	query := `INSERT INTO likes (tweet_id, user_id, created_at) VALUES ($1, $2, $3)`
	_, err := s.db.Exec(query, like.TweetID, like.UserID, like.CreatedAt)
	return err
}

// ======================================================================
= Seed Retweets
// ======================================================================

func (s *Seeder) seedRetweets() error {
	if *retweets <= 0 || len(s.tweets) == 0 {
		return nil
	}
	log.Printf("Seeding %d retweets...", *retweets)

	retweetedCount := 0
	for i := 0; i < *retweets; i++ {
		tweet := s.tweets[s.rand.Intn(len(s.tweets))]
		user := s.users[s.rand.Intn(len(s.users))]

		// Check if already retweeted
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM retweets WHERE tweet_id = $1 AND user_id = $2)`
		_ = s.db.QueryRow(query, tweet.ID, user.ID).Scan(&exists)
		if exists {
			continue
		}

		retweet := &Retweet{
			TweetID:   tweet.ID,
			UserID:    user.ID,
			CreatedAt: randomPastTime(5),
		}
		if err := s.insertRetweet(retweet); err != nil {
			continue
		}
		retweetedCount++
	}

	log.Printf("Seeded %d retweets", retweetedCount)
	return nil
}

func (s *Seeder) insertRetweet(retweet *Retweet) error {
	query := `INSERT INTO retweets (tweet_id, user_id, created_at) VALUES ($1, $2, $3)`
	_, err := s.db.Exec(query, retweet.TweetID, retweet.UserID, retweet.CreatedAt)
	return err
}

// ======================================================================
= Seed Bookmarks
// ======================================================================

func (s *Seeder) seedBookmarks() error {
	if *bookmarks <= 0 || len(s.tweets) == 0 {
		return nil
	}
	log.Printf("Seeding %d bookmarks...", *bookmarks)

	bookmarkedCount := 0
	for i := 0; i < *bookmarks; i++ {
		tweet := s.tweets[s.rand.Intn(len(s.tweets))]
		user := s.users[s.rand.Intn(len(s.users))]

		// Check if already bookmarked
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM bookmarks WHERE tweet_id = $1 AND user_id = $2)`
		_ = s.db.QueryRow(query, tweet.ID, user.ID).Scan(&exists)
		if exists {
			continue
		}

		bookmark := &Bookmark{
			TweetID:   tweet.ID,
			UserID:    user.ID,
			CreatedAt: randomPastTime(3),
		}
		if err := s.insertBookmark(bookmark); err != nil {
			continue
		}
		bookmarkedCount++
	}

	log.Printf("Seeded %d bookmarks", bookmarkedCount)
	return nil
}

func (s *Seeder) insertBookmark(bookmark *Bookmark) error {
	query := `INSERT INTO bookmarks (tweet_id, user_id, created_at) VALUES ($1, $2, $3)`
	_, err := s.db.Exec(query, bookmark.TweetID, bookmark.UserID, bookmark.CreatedAt)
	return err
}

// ======================================================================
= Seed Messages
// ======================================================================

func (s *Seeder) seedMessages() error {
	if *messages <= 0 || len(s.users) < 2 {
		return nil
	}
	log.Printf("Seeding %d messages...", *messages)

	messageContent := []string{
		"Hey, how are you doing?",
		"Great project! Keep up the good work!",
		"Can you help me with this issue?",
		"Thanks for the feedback!",
		"Let's collaborate on this idea.",
		"Check out my latest commit.",
		"Coming to the meetup tonight?",
		"Have you seen the new feature?",
		"Code review requested. Please take a look.",
		"Congratulations on the launch!",
		"Would love your input on this.",
		"Meeting at 3 PM in the virtual room.",
		"Documentation updated. Please review.",
		"Great presentation yesterday!",
		"Let's schedule a call to discuss.",
		"Can you share the link?",
		"Thanks for the quick response!",
		"Working on it now.",
		"Let me know if you need any help.",
		"Excellent work!",
	}

	sentCount := 0
	for i := 0; i < *messages; i++ {
		sender := s.users[s.rand.Intn(len(s.users))]
		receiver := s.users[s.rand.Intn(len(s.users))]
		if sender.ID == receiver.ID {
			continue
		}

		content := messageContent[s.rand.Intn(len(messageContent))]
		msg := &Message{
			ID:         uuid.New().String(),
			SenderID:   sender.ID,
			ReceiverID: receiver.ID,
			Content:    content,
			Read:       s.rand.Intn(2) == 0,
			CreatedAt:  randomPastTime(7),
		}
		if err := s.insertMessage(msg); err != nil {
			continue
		}
		sentCount++
	}

	log.Printf("Seeded %d messages", sentCount)
	return nil
}

func (s *Seeder) insertMessage(msg *Message) error {
	query := `
		INSERT INTO messages (id, sender_id, receiver_id, content, read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := s.db.Exec(query,
		msg.ID, msg.SenderID, msg.ReceiverID, msg.Content,
		msg.Read, msg.CreatedAt,
	)
	return err
}

// ======================================================================
= Seed Notifications
// ======================================================================

func (s *Seeder) seedNotifications() error {
	if len(s.users) < 2 {
		return nil
	}
	log.Printf("Seeding notifications...")

	notificationTypes := []string{"like", "retweet", "follow", "reply", "mention"}

	// Create some notifications for each user
	for _, user := range s.users {
		numNotifs := 1 + s.rand.Intn(5)
		for i := 0; i < numNotifs; i++ {
			fromUser := s.users[s.rand.Intn(len(s.users))]
			if fromUser.ID == user.ID {
				continue
			}
			notifType := notificationTypes[s.rand.Intn(len(notificationTypes))]
			refID := uuid.New().String()
			if notifType == "like" || notifType == "retweet" || notifType == "reply" {
				if len(s.tweets) > 0 {
					refID = s.tweets[s.rand.Intn(len(s.tweets))].ID
				}
			}
			notification := &Notification{
				ID:          uuid.New().String(),
				UserID:      user.ID,
				FromUserID:  fromUser.ID,
				Type:        notifType,
				ReferenceID: refID,
				Read:        s.rand.Intn(2) == 0,
				CreatedAt:   randomPastTime(10),
			}
			if err := s.insertNotification(notification); err != nil {
				continue
			}
		}
	}

	return nil
}

func (s *Seeder) insertNotification(notif *Notification) error {
	query := `
		INSERT INTO notifications (id, user_id, from_user_id, type, reference_id, read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.Exec(query,
		notif.ID, notif.UserID, notif.FromUserID, notif.Type,
		notif.ReferenceID, notif.Read, notif.CreatedAt,
	)
	return err
}

// ======================================================================
= Seed Polls
// ======================================================================

func (s *Seeder) seedPolls() error {
	if *polls <= 0 || len(s.tweets) == 0 {
		return nil
	}
	log.Printf("Seeding %d polls...", *polls)

	pollOptions := [][]string{
		{"Option A", "Option B"},
		{"Yes", "No"},
		{"Option 1", "Option 2", "Option 3"},
		{"Agree", "Disagree", "Neutral"},
		{"Python", "Go", "Rust", "JavaScript"},
		{"React", "Vue", "Angular", "Svelte"},
		{"PostgreSQL", "MySQL", "MongoDB", "Redis"},
		{"AWS", "Azure", "GCP", "DigitalOcean"},
		{"Docker", "Podman", "Containerd"},
		{"Kubernetes", "Nomad", "Swarm"},
	}

	polled := 0
	for i := 0; i < *polls && i < len(s.tweets); i++ {
		tweet := s.tweets[s.rand.Intn(len(s.tweets))]
		options := pollOptions[s.rand.Intn(len(pollOptions))]
		duration := time.Duration(1+s.rand.Intn(7)) * 24 * time.Hour

		poll := &Poll{
			ID:        uuid.New().String(),
			TweetID:   tweet.ID,
			Options:   options,
			ExpiresAt: time.Now().Add(duration),
			CreatedAt: tweet.CreatedAt,
		}
		if err := s.insertPoll(poll); err != nil {
			continue
		}
		polled++
	}

	log.Printf("Seeded %d polls", polled)
	return nil
}

func (s *Seeder) insertPoll(poll *Poll) error {
	optionsJSON, err := json.Marshal(poll.Options)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO polls (id, tweet_id, options, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = s.db.Exec(query, poll.ID, poll.TweetID, optionsJSON, poll.ExpiresAt, poll.CreatedAt)
	return err
}

// ======================================================================
= Seed Reports
// ======================================================================

func (s *Seeder) seedReports() error {
	if *reports <= 0 || len(s.users) < 2 || len(s.tweets) == 0 {
		return nil
	}
	log.Printf("Seeding %d reports...", *reports)

	reportReasons := []string{
		"Spam content",
		"Harassment",
		"Inappropriate content",
		"Hate speech",
		"Misleading information",
		"Self-harm",
		"Violence",
		"Nudity",
		"Impersonation",
		"Copyright violation",
	}

	statuses := []string{"pending", "under_review", "resolved", "dismissed"}
	severities := []string{"low", "medium", "high", "critical"}

	for i := 0; i < *reports; i++ {
		reporter := s.users[s.rand.Intn(len(s.users))]
		tweet := s.tweets[s.rand.Intn(len(s.tweets))]

		report := &Report{
			ID:          uuid.New().String(),
			ReporterID:  reporter.ID,
			TargetID:    tweet.ID,
			TargetType:  "tweet",
			Reason:      reportReasons[s.rand.Intn(len(reportReasons))],
			Status:      statuses[s.rand.Intn(len(statuses))],
			Severity:    severities[s.rand.Intn(len(severities))],
			CreatedAt:   randomPastTime(5),
		}
		if err := s.insertReport(report); err != nil {
			continue
		}
	}

	log.Printf("Seeded %d reports", *reports)
	return nil
}

func (s *Seeder) insertReport(report *Report) error {
	query := `
		INSERT INTO reports (id, reporter_id, target_id, target_type, reason, status, severity, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := s.db.Exec(query,
		report.ID, report.ReporterID, report.TargetID, report.TargetType,
		report.Reason, report.Status, report.Severity, report.CreatedAt,
	)
	return err
}

// ======================================================================
= Clean Database
// ======================================================================

func cleanDatabase(db *sql.DB) error {
	tables := []string{
		"reports", "community_members", "communities", "polls",
		"messages", "bookmarks", "retweets", "likes", "follows",
		"notifications", "tweets", "users", "sessions",
	}

	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to truncate %s: %w", table, err)
		}
		if *verbose {
			log.Printf("Truncated table: %s", table)
		}
	}
	return nil
}

// ======================================================================
= Helper Functions
// ======================================================================

func randomPastTime(maxDays int) time.Time {
	days := rand.Intn(maxDays)
	hours := rand.Intn(24)
	minutes := rand.Intn(60)
	return time.Now().AddDate(0, 0, -days).Add(-time.Duration(hours) * time.Hour).Add(-time.Duration(minutes) * time.Minute)
}

func randomBio() string {
	bios := []string{
		"Software engineer passionate about building great products",
		"Full-stack developer | Go, React, PostgreSQL",
		"DevOps engineer and cloud architect",
		"Product designer and UX enthusiast",
		"Data scientist and AI researcher",
		"Open source contributor and community builder",
		"Security engineer and ethical hacker",
		"Technical writer and documentation advocate",
		"Machine learning engineer and researcher",
		"Frontend specialist and UI/UX designer",
		"Backend engineer and systems architect",
		"Mobile developer and app creator",
		"Blockchain developer and crypto enthusiast",
		"Game developer and VR explorer",
		"Robotics engineer and automation expert",
		"Bioinformatics researcher and data analyst",
	}
	return bios[rand.Intn(len(bios))]
}