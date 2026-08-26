-- backend/sqlc/queries.sql
-- sqlc queries for twitter clone database

-- ======================================================================
-- name: CreateUser :exec
-- ======================================================================
INSERT INTO users (
    id, username, email, password_hash, full_name, bio,
    avatar_url, banner_url, location, website, role, status,
    is_verified, is_private, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
);

-- ======================================================================
-- name: GetUserByID :one
-- ======================================================================
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- ======================================================================
-- name: GetUserByUsername :one
-- ======================================================================
SELECT * FROM users WHERE username = $1 AND deleted_at IS NULL;

-- ======================================================================
-- name: GetUserByEmail :one
-- ======================================================================
SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL;

-- ======================================================================
-- name: GetUserByUsernameOrEmail :one
-- ======================================================================
SELECT * FROM users 
WHERE (username = $1 OR email = $2) AND deleted_at IS NULL;

-- ======================================================================
-- name: ListUsers :many
-- ======================================================================
SELECT * FROM users 
WHERE deleted_at IS NULL 
ORDER BY created_at DESC 
LIMIT $1 OFFSET $2;

-- ======================================================================
-- name: CountUsers :one
-- ======================================================================
SELECT COUNT(*) FROM users WHERE deleted_at IS NULL;

-- ======================================================================
-- name: UpdateUser :exec
-- ======================================================================
UPDATE users SET
    username = $1,
    email = $2,
    password_hash = $3,
    full_name = $4,
    bio = $5,
    avatar_url = $6,
    banner_url = $7,
    location = $8,
    website = $9,
    role = $10,
    status = $11,
    is_verified = $12,
    is_private = $13,
    updated_at = $14
WHERE id = $15 AND deleted_at IS NULL;

-- ======================================================================
-- name: SoftDeleteUser :exec
-- ======================================================================
UPDATE users SET deleted_at = NOW() WHERE id = $1;

-- ======================================================================
-- name: HardDeleteUser :exec
-- ======================================================================
DELETE FROM users WHERE id = $1;

-- ======================================================================
-- name: IncrementUserTweetCount :exec
-- ======================================================================
UPDATE users SET tweet_count = tweet_count + 1 WHERE id = $1;

-- ======================================================================
-- name: DecrementUserTweetCount :exec
-- ======================================================================
UPDATE users SET tweet_count = GREATEST(tweet_count - 1, 0) WHERE id = $1;

-- ======================================================================
-- name: IncrementUserFollowerCount :exec
-- ======================================================================
UPDATE users SET follower_count = follower_count + 1 WHERE id = $1;

-- ======================================================================
-- name: DecrementUserFollowerCount :exec
-- ======================================================================
UPDATE users SET follower_count = GREATEST(follower_count - 1, 0) WHERE id = $1;

-- ======================================================================
-- name: IncrementUserFollowingCount :exec
-- ======================================================================
UPDATE users SET following_count = following_count + 1 WHERE id = $1;

-- ======================================================================
-- name: DecrementUserFollowingCount :exec
-- ======================================================================
UPDATE users SET following_count = GREATEST(following_count - 1, 0) WHERE id = $1;

-- ======================================================================
-- name: UpdateUserLastActive :exec
-- ======================================================================
UPDATE users SET last_active = NOW() WHERE id = $1;

-- ======================================================================
-- name: SearchUsers :many
-- ======================================================================
SELECT * FROM users 
WHERE deleted_at IS NULL 
  AND (username ILIKE '%' || $1 || '%' 
    OR full_name ILIKE '%' || $1 || '%' 
    OR bio ILIKE '%' || $1 || '%')
ORDER BY 
    CASE 
        WHEN username ILIKE $1 THEN 1
        WHEN full_name ILIKE $1 THEN 2
        ELSE 3
    END,
    username
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: GetUsersByIDs :many
-- ======================================================================
SELECT * FROM users 
WHERE id = ANY($1::text[]) AND deleted_at IS NULL;

-- ======================================================================
-- name: GetUserStats :one
-- ======================================================================
SELECT 
    u.id AS user_id,
    u.tweet_count,
    u.follower_count,
    u.following_count,
    COUNT(DISTINCT l.id) AS total_likes,
    COUNT(DISTINCT rt.id) AS total_retweets,
    COUNT(DISTINCT r.id) AS total_replies,
    COUNT(DISTINCT b.id) AS total_bookmarks,
    u.created_at AS joined_at,
    u.last_active
FROM users u
LEFT JOIN tweets t ON t.user_id = u.id AND t.deleted_at IS NULL
LEFT JOIN likes l ON l.tweet_id = t.id
LEFT JOIN retweets rt ON rt.tweet_id = t.id
LEFT JOIN tweets r ON r.parent_tweet_id = t.id AND r.deleted_at IS NULL
LEFT JOIN bookmarks b ON b.tweet_id = t.id
WHERE u.id = $1 AND u.deleted_at IS NULL
GROUP BY u.id;

-- ======================================================================
-- name: CreateTweet :exec
-- ======================================================================
INSERT INTO tweets (
    id, user_id, content, media_urls, parent_tweet_id,
    retweet_of_id, is_poll, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
);

-- ======================================================================
-- name: GetTweetByID :one
-- ======================================================================
SELECT * FROM tweets WHERE id = $1 AND deleted_at IS NULL;

-- ======================================================================
-- name: GetTweetsByIDs :many
-- ======================================================================
SELECT * FROM tweets WHERE id = ANY($1::text[]) AND deleted_at IS NULL;

-- ======================================================================
-- name: UpdateTweet :exec
-- ======================================================================
UPDATE tweets SET
    content = $1,
    media_urls = $2,
    updated_at = $3
WHERE id = $4 AND deleted_at IS NULL;

-- ======================================================================
-- name: SoftDeleteTweet :exec
-- ======================================================================
UPDATE tweets SET deleted_at = NOW() WHERE id = $1;

-- ======================================================================
-- name: HardDeleteTweet :exec
-- ======================================================================
DELETE FROM tweets WHERE id = $1;

-- ======================================================================
-- name: RestoreTweet :exec
-- ======================================================================
UPDATE tweets SET deleted_at = NULL WHERE id = $1;

-- ======================================================================
-- name: GetTweetsByUserID :many
-- ======================================================================
SELECT * FROM tweets 
WHERE user_id = $1 AND deleted_at IS NULL 
ORDER BY created_at DESC 
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: GetTweetsByUserIDWithReplies :many
-- ======================================================================
SELECT * FROM tweets 
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC 
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: CountTweetsByUserID :one
-- ======================================================================
SELECT COUNT(*) FROM tweets WHERE user_id = $1 AND deleted_at IS NULL;

-- ======================================================================
-- name: GetFeedTweets :many
-- ======================================================================
SELECT t.* FROM tweets t
WHERE t.user_id = ANY($1::text[]) 
  AND t.deleted_at IS NULL
  AND t.parent_tweet_id IS NULL
ORDER BY t.created_at DESC
LIMIT $2;

-- ======================================================================
-- name: GetFeedTweetsWithCursor :many
-- ======================================================================
SELECT t.* FROM tweets t
WHERE t.user_id = ANY($1::text[]) 
  AND t.deleted_at IS NULL
  AND t.parent_tweet_id IS NULL
  AND t.created_at < $2
ORDER BY t.created_at DESC
LIMIT $3;

-- ======================================================================
-- name: GetReplies :many
-- ======================================================================
SELECT * FROM tweets 
WHERE parent_tweet_id = $1 AND deleted_at IS NULL
ORDER BY created_at ASC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: CountReplies :one
-- ======================================================================
SELECT COUNT(*) FROM tweets WHERE parent_tweet_id = $1 AND deleted_at IS NULL;

-- ======================================================================
-- name: GetRetweetsOfTweet :many
-- ======================================================================
SELECT * FROM tweets 
WHERE retweet_of_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: CountRetweetsOfTweet :one
-- ======================================================================
SELECT COUNT(*) FROM tweets WHERE retweet_of_id = $1 AND deleted_at IS NULL;

-- ======================================================================
-- name: SearchTweets :many
-- ======================================================================
SELECT t.*, 
       ts_rank(to_tsvector('english', t.content), plainto_tsquery('english', $1)) AS rank
FROM tweets t
WHERE to_tsvector('english', t.content) @@ plainto_tsquery('english', $1)
  AND t.deleted_at IS NULL
ORDER BY rank DESC, t.created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: GetTrendingHashtags :many
-- ======================================================================
SELECT 
    LOWER(SUBSTRING(t.content FROM '#([A-Za-z0-9_]+)')) AS hashtag,
    COUNT(*) AS count,
    MIN(t.created_at) AS first_seen,
    MAX(t.created_at) AS last_seen
FROM tweets t
WHERE t.content ~ '#[A-Za-z0-9_]+'
  AND t.created_at > NOW() - INTERVAL '24 hours'
  AND t.deleted_at IS NULL
GROUP BY hashtag
ORDER BY count DESC
LIMIT $1;

-- ======================================================================
-- name: GetMostLikedTweets :many
-- ======================================================================
SELECT t.*, COUNT(l.id) AS like_count
FROM tweets t
LEFT JOIN likes l ON t.id = l.tweet_id
WHERE t.deleted_at IS NULL
  AND t.created_at > $1
GROUP BY t.id
ORDER BY like_count DESC, t.created_at DESC
LIMIT $2;

-- ======================================================================
-- name: GetMostRetweetedTweets :many
-- ======================================================================
SELECT t.*, COUNT(rt.id) AS retweet_count
FROM tweets t
LEFT JOIN retweets rt ON t.id = rt.tweet_id
WHERE t.deleted_at IS NULL
  AND t.created_at > $1
GROUP BY t.id
ORDER BY retweet_count DESC, t.created_at DESC
LIMIT $2;

-- ======================================================================
-- name: CreateFollow :exec
-- ======================================================================
INSERT INTO follows (follower_id, followee_id, status, created_at) 
VALUES ($1, $2, 'accepted', NOW());

-- ======================================================================
-- name: DeleteFollow :exec
-- ======================================================================
DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2;

-- ======================================================================
-- name: FollowExists :one
-- ======================================================================
SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2);

-- ======================================================================
-- name: CountFollowers :one
-- ======================================================================
SELECT COUNT(*) FROM follows WHERE followee_id = $1;

-- ======================================================================
-- name: CountFollowing :one
-- ======================================================================
SELECT COUNT(*) FROM follows WHERE follower_id = $1;

-- ======================================================================
-- name: CountMutualFollows :one
-- ======================================================================
SELECT COUNT(*)
FROM follows f1
JOIN follows f2 ON f1.followee_id = f2.follower_id
WHERE f1.follower_id = $1 AND f2.followee_id = $2;

-- ======================================================================
-- name: GetFollowers :many
-- ======================================================================
SELECT f.* FROM follows f
WHERE f.followee_id = $1
ORDER BY f.created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: GetFollowing :many
-- ======================================================================
SELECT f.* FROM follows f
WHERE f.follower_id = $1
ORDER BY f.created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: AreMutual :one
-- ======================================================================
SELECT EXISTS(
    SELECT 1 FROM follows f1
    JOIN follows f2 ON f1.followee_id = f2.follower_id
    WHERE f1.follower_id = $1 AND f2.followee_id = $2
);

-- ======================================================================
-- name: GetFollowRecommendations :many
-- ======================================================================
SELECT DISTINCT f2.followee_id
FROM follows f1
JOIN follows f2 ON f1.followee_id = f2.follower_id
WHERE f1.follower_id = $1
  AND f2.followee_id != $1
  AND f2.followee_id NOT IN (
    SELECT followee_id FROM follows WHERE follower_id = $1
  )
ORDER BY RANDOM()
LIMIT $2;

-- ======================================================================
-- name: CreateLike :exec
-- ======================================================================
INSERT INTO likes (tweet_id, user_id, type, created_at) 
VALUES ($1, $2, $3, NOW());

-- ======================================================================
-- name: DeleteLike :exec
-- ======================================================================
DELETE FROM likes WHERE tweet_id = $1 AND user_id = $2;

-- ======================================================================
-- name: LikeExists :one
-- ======================================================================
SELECT EXISTS(SELECT 1 FROM likes WHERE tweet_id = $1 AND user_id = $2);

-- ======================================================================
-- name: CountLikes :one
-- ======================================================================
SELECT COUNT(*) FROM likes WHERE tweet_id = $1;

-- ======================================================================
-- name: CountLikesByUser :one
-- ======================================================================
SELECT COUNT(*) FROM likes WHERE user_id = $1;

-- ======================================================================
-- name: GetLikesByTweetID :many
-- ======================================================================
SELECT l.*, u.username, u.full_name, u.avatar_url
FROM likes l
JOIN users u ON l.user_id = u.id
WHERE l.tweet_id = $1
ORDER BY l.created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: GetLikesByUserID :many
-- ======================================================================
SELECT l.* FROM likes l
WHERE l.user_id = $1
ORDER BY l.created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: CreateRetweet :exec
-- ======================================================================
INSERT INTO retweets (tweet_id, user_id, type, quote_content, created_at) 
VALUES ($1, $2, $3, $4, NOW());

-- ======================================================================
-- name: DeleteRetweet :exec
-- ======================================================================
DELETE FROM retweets WHERE tweet_id = $1 AND user_id = $2;

-- ======================================================================
-- name: RetweetExists :one
-- ======================================================================
SELECT EXISTS(SELECT 1 FROM retweets WHERE tweet_id = $1 AND user_id = $2);

-- ======================================================================
-- name: CountRetweets :one
-- ======================================================================
SELECT COUNT(*) FROM retweets WHERE tweet_id = $1;

-- ======================================================================
-- name: CountRetweetsByUser :one
-- ======================================================================
SELECT COUNT(*) FROM retweets WHERE user_id = $1;

-- ======================================================================
-- name: GetRetweetsByTweetID :many
-- ======================================================================
SELECT rt.*, u.username, u.full_name, u.avatar_url
FROM retweets rt
JOIN users u ON rt.user_id = u.id
WHERE rt.tweet_id = $1
ORDER BY rt.created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: GetRetweetsByUserID :many
-- ======================================================================
SELECT rt.* FROM retweets rt
WHERE rt.user_id = $1
ORDER BY rt.created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: CreateBookmark :exec
-- ======================================================================
INSERT INTO bookmarks (tweet_id, user_id, category, name, notes, created_at, updated_at) 
VALUES ($1, $2, $3, $4, $5, NOW(), NOW());

-- ======================================================================
-- name: DeleteBookmark :exec
-- ======================================================================
DELETE FROM bookmarks WHERE tweet_id = $1 AND user_id = $2;

-- ======================================================================
-- name: BookmarkExists :one
-- ======================================================================
SELECT EXISTS(SELECT 1 FROM bookmarks WHERE tweet_id = $1 AND user_id = $2);

-- ======================================================================
-- name: CountBookmarks :one
-- ======================================================================
SELECT COUNT(*) FROM bookmarks WHERE tweet_id = $1;

-- ======================================================================
-- name: CountBookmarksByUser :one
-- ======================================================================
SELECT COUNT(*) FROM bookmarks WHERE user_id = $1;

-- ======================================================================
-- name: GetBookmarksByUserID :many
-- ======================================================================
SELECT b.*, t.content, t.created_at AS tweet_created_at,
       u.username, u.full_name, u.avatar_url
FROM bookmarks b
JOIN tweets t ON b.tweet_id = t.id
JOIN users u ON t.user_id = u.id
WHERE b.user_id = $1
  AND t.deleted_at IS NULL
ORDER BY b.created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: CreateMessage :exec
-- ======================================================================
INSERT INTO messages (
    id, sender_id, receiver_id, content, media_urls,
    read, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- ======================================================================
-- name: GetMessageByID :one
-- ======================================================================
SELECT * FROM messages WHERE id = $1 AND deleted_at IS NULL;

-- ======================================================================
-- name: GetMessagesBetweenUsers :many
-- ======================================================================
SELECT * FROM messages
WHERE (sender_id = $1 AND receiver_id = $2) 
   OR (sender_id = $2 AND receiver_id = $1)
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- ======================================================================
-- name: GetConversations :many
-- ======================================================================
SELECT 
    CASE 
        WHEN sender_id = $1 THEN receiver_id
        WHEN receiver_id = $1 THEN sender_id
    END AS other_user_id,
    MAX(created_at) AS last_message_at,
    (SELECT id FROM messages m2 
     WHERE (m2.sender_id = $1 AND m2.receiver_id = other_user_id)
        OR (m2.sender_id = other_user_id AND m2.receiver_id = $1)
       AND m2.deleted_at IS NULL
     ORDER BY m2.created_at DESC LIMIT 1) AS last_message_id,
    (SELECT content FROM messages m3 WHERE m3.id = last_message_id) AS last_message_content,
    (SELECT read FROM messages m3 WHERE m3.id = last_message_id) AS last_message_read,
    COUNT(CASE WHEN read = false AND receiver_id = $1 THEN 1 END) AS unread_count
FROM messages
WHERE (sender_id = $1 OR receiver_id = $1)
  AND deleted_at IS NULL
GROUP BY other_user_id
ORDER BY last_message_at DESC;

-- ======================================================================
-- name: CountUnreadMessages :one
-- ======================================================================
SELECT COUNT(*) FROM messages 
WHERE receiver_id = $1 AND read = false AND deleted_at IS NULL;

-- ======================================================================
-- name: CountUnreadFromUser :one
-- ======================================================================
SELECT COUNT(*) FROM messages 
WHERE receiver_id = $1 AND sender_id = $2 AND read = false AND deleted_at IS NULL;

-- ======================================================================
-- name: MarkMessageRead :exec
-- ======================================================================
UPDATE messages SET read = true, read_at = NOW() WHERE id = $1;

-- ======================================================================
-- name: MarkConversationRead :exec
-- ======================================================================
UPDATE messages 
SET read = true, read_at = NOW()
WHERE receiver_id = $1 AND sender_id = $2 AND read = false;

-- ======================================================================
-- name: MarkAllMessagesRead :exec
-- ======================================================================
UPDATE messages 
SET read = true, read_at = NOW()
WHERE receiver_id = $1 AND read = false;

-- ======================================================================
-- name: SoftDeleteMessage :exec
-- ======================================================================
UPDATE messages SET deleted_at = NOW() WHERE id = $1;

-- ======================================================================
-- name: HardDeleteMessage :exec
-- ======================================================================
DELETE FROM messages WHERE id = $1;

-- ======================================================================
-- name: DeleteConversation :exec
-- ======================================================================
DELETE FROM messages 
WHERE (sender_id = $1 AND receiver_id = $2) 
   OR (sender_id = $2 AND receiver_id = $1);

-- ======================================================================
-- name: SearchMessages :many
-- ======================================================================
SELECT * FROM messages
WHERE (sender_id = $1 OR receiver_id = $1)
  AND content ILIKE '%' || $2 || '%'
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- ======================================================================
-- name: CreateNotification :exec
-- ======================================================================
INSERT INTO notifications (
    id, user_id, from_user_id, type, reference_id, read, created_at
) VALUES ($1, $2, $3, $4, $5, $6, NOW());

-- ======================================================================
-- name: GetNotificationByID :one
-- ======================================================================
SELECT * FROM notifications WHERE id = $1;

-- ======================================================================
-- name: GetNotificationsByUserID :many
-- ======================================================================
SELECT * FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: GetUnreadNotifications :many
-- ======================================================================
SELECT * FROM notifications
WHERE user_id = $1 AND read = false
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: CountUnreadNotifications :one
-- ======================================================================
SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = false;

-- ======================================================================
-- name: CountNotificationsByUser :one
-- ======================================================================
SELECT COUNT(*) FROM notifications WHERE user_id = $1;

-- ======================================================================
-- name: MarkNotificationRead :exec
-- ======================================================================
UPDATE notifications SET read = true, read_at = NOW() WHERE id = $1;

-- ======================================================================
-- name: MarkAllNotificationsRead :exec
-- ======================================================================
UPDATE notifications SET read = true, read_at = NOW() WHERE user_id = $1 AND read = false;

-- ======================================================================
-- name: MarkMultipleNotificationsRead :exec
-- ======================================================================
UPDATE notifications SET read = true, read_at = NOW() 
WHERE id = ANY($1::text[]);

-- ======================================================================
-- name: DeleteNotification :exec
-- ======================================================================
DELETE FROM notifications WHERE id = $1;

-- ======================================================================
-- name: DeleteAllNotifications :exec
-- ======================================================================
DELETE FROM notifications WHERE user_id = $1;

-- ======================================================================
-- name: GetNotificationsByType :many
-- ======================================================================
SELECT * FROM notifications
WHERE user_id = $1 AND type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- ======================================================================
-- name: CountNotificationsByType :one
-- ======================================================================
SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND type = $2;

-- ======================================================================
-- name: GroupNotificationsByType :many
-- ======================================================================
SELECT 
    type,
    COUNT(*) AS count,
    MAX(created_at) AS latest_at,
    SUM(CASE WHEN read = true THEN 1 ELSE 0 END) AS read_count,
    SUM(CASE WHEN read = false THEN 1 ELSE 0 END) AS unread_count
FROM notifications
WHERE user_id = $1
GROUP BY type
ORDER BY latest_at DESC;

-- ======================================================================
-- name: CreatePoll :exec
-- ======================================================================
INSERT INTO polls (
    id, tweet_id, options, duration, expires_at, created_at
) VALUES ($1, $2, $3, $4, $5, NOW());

-- ======================================================================
-- name: GetPollByID :one
-- ======================================================================
SELECT * FROM polls WHERE id = $1;

-- ======================================================================
-- name: GetPollByTweetID :one
-- ======================================================================
SELECT * FROM polls WHERE tweet_id = $1;

-- ======================================================================
-- name: UpdatePoll :exec
-- ======================================================================
UPDATE polls SET options = $1, updated_at = NOW() WHERE id = $2;

-- ======================================================================
-- name: DeletePoll :exec
-- ======================================================================
DELETE FROM polls WHERE id = $1;

-- ======================================================================
-- name: GetExpiredPolls :many
-- ======================================================================
SELECT * FROM polls WHERE expires_at <= NOW() AND deleted_at IS NULL;

-- ======================================================================
-- name: CreateCommunity :exec
-- ======================================================================
INSERT INTO communities (
    id, name, slug, description, avatar_url, banner_url,
    created_by, is_private, member_count, post_count, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW());

-- ======================================================================
-- name: GetCommunityByID :one
-- ======================================================================
SELECT * FROM communities WHERE id = $1 AND deleted_at IS NULL;

-- ======================================================================
-- name: GetCommunityBySlug :one
-- ======================================================================
SELECT * FROM communities WHERE slug = $1 AND deleted_at IS NULL;

-- ======================================================================
-- name: UpdateCommunity :exec
-- ======================================================================
UPDATE communities SET
    name = $1,
    description = $2,
    avatar_url = $3,
    banner_url = $4,
    is_private = $5,
    updated_at = NOW()
WHERE id = $6 AND deleted_at IS NULL;

-- ======================================================================
-- name: SoftDeleteCommunity :exec
-- ======================================================================
UPDATE communities SET deleted_at = NOW() WHERE id = $1;

-- ======================================================================
-- name: ListCommunities :many
-- ======================================================================
SELECT * FROM communities
WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- ======================================================================
-- name: SearchCommunities :many
-- ======================================================================
SELECT * FROM communities
WHERE deleted_at IS NULL
  AND (name ILIKE '%' || $1 || '%' 
    OR description ILIKE '%' || $1 || '%')
ORDER BY name
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: AddCommunityMember :exec
-- ======================================================================
INSERT INTO community_members (community_id, user_id, role, joined_at)
VALUES ($1, $2, $3, NOW());

-- ======================================================================
-- name: RemoveCommunityMember :exec
-- ======================================================================
DELETE FROM community_members WHERE community_id = $1 AND user_id = $2;

-- ======================================================================
-- name: UpdateCommunityMemberRole :exec
-- ======================================================================
UPDATE community_members SET role = $1, updated_at = NOW() 
WHERE community_id = $2 AND user_id = $3;

-- ======================================================================
-- name: GetCommunityMemberRole :one
-- ======================================================================
SELECT role FROM community_members 
WHERE community_id = $1 AND user_id = $2;

-- ======================================================================
-- name: IsCommunityMember :one
-- ======================================================================
SELECT EXISTS(SELECT 1 FROM community_members 
WHERE community_id = $1 AND user_id = $2);

-- ======================================================================
-- name: IsCommunityAdmin :one
-- ======================================================================
SELECT EXISTS(SELECT 1 FROM community_members 
WHERE community_id = $1 AND user_id = $2 AND role IN ('admin', 'owner'));

-- ======================================================================
-- name: GetCommunityMembers :many
-- ======================================================================
SELECT cm.*, u.username, u.full_name, u.avatar_url
FROM community_members cm
JOIN users u ON cm.user_id = u.id
WHERE cm.community_id = $1
ORDER BY cm.joined_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: GetUserCommunities :many
-- ======================================================================
SELECT c.*
FROM communities c
JOIN community_members cm ON c.id = cm.community_id
WHERE cm.user_id = $1 AND c.deleted_at IS NULL
ORDER BY cm.joined_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: AddCommunityPost :exec
-- ======================================================================
INSERT INTO community_posts (community_id, tweet_id, created_at)
VALUES ($1, $2, NOW());

-- ======================================================================
-- name: RemoveCommunityPost :exec
-- ======================================================================
DELETE FROM community_posts WHERE community_id = $1 AND tweet_id = $2;

-- ======================================================================
-- name: GetCommunityPosts :many
-- ======================================================================
SELECT t.*
FROM community_posts cp
JOIN tweets t ON cp.tweet_id = t.id
WHERE cp.community_id = $1 AND t.deleted_at IS NULL
ORDER BY cp.created_at DESC
LIMIT $2 OFFSET $3;

-- ======================================================================
-- name: CreateReport :exec
-- ======================================================================
INSERT INTO reports (
    id, reporter_id, target_id, target_type, reason,
    description, status, severity, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW());

-- ======================================================================
-- name: GetReportByID :one
-- ======================================================================
SELECT * FROM reports WHERE id = $1;

-- ======================================================================
-- name: ListReports :many
-- ======================================================================
SELECT * FROM reports
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- ======================================================================
-- name: UpdateReportStatus :exec
-- ======================================================================
UPDATE reports SET
    status = $1,
    reviewer_id = $2,
    review_notes = $3,
    resolved_at = NOW(),
    updated_at = NOW()
WHERE id = $4;

-- ======================================================================
-- name: CountReportsByStatus :one
-- ======================================================================
SELECT COUNT(*) FROM reports WHERE status = $1;

-- ======================================================================
-- name: CreateSession :exec
-- ======================================================================
INSERT INTO sessions (
    id, user_id, refresh_token, user_agent, ip,
    expires_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW());

-- ======================================================================
-- name: GetSessionByID :one
-- ======================================================================
SELECT * FROM sessions WHERE id = $1;

-- ======================================================================
-- name: GetSessionByRefreshToken :one
-- ======================================================================
SELECT * FROM sessions WHERE refresh_token = $1;

-- ======================================================================
-- name: GetSessionsByUserID :many
-- ======================================================================
SELECT * FROM sessions 
WHERE user_id = $1 
ORDER BY created_at DESC;

-- ======================================================================
-- name: UpdateSession :exec
-- ======================================================================
UPDATE sessions SET
    refresh_token = $1,
    expires_at = $2,
    updated_at = NOW()
WHERE id = $3;

-- ======================================================================
-- name: DeleteSession :exec
-- ======================================================================
DELETE FROM sessions WHERE id = $1;

-- ======================================================================
-- name: DeleteSessionsByUserID :exec
-- ======================================================================
DELETE FROM sessions WHERE user_id = $1;

-- ======================================================================
-- name: CleanupExpiredSessions :exec
-- ======================================================================
DELETE FROM sessions WHERE expires_at <= NOW();

-- ======================================================================
-- name: CreateSpace :exec
-- ======================================================================
INSERT INTO spaces (
    id, title, description, topic, created_by,
    visibility, type, status, scheduled_at, duration,
    max_listeners, invite_code, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW());

-- ======================================================================
-- name: GetSpaceByID :one
-- ======================================================================
SELECT * FROM spaces WHERE id = $1 AND deleted_at IS NULL;

-- ======================================================================
-- name: ListSpaces :many
-- ======================================================================
SELECT * FROM spaces
WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- ======================================================================
-- name: UpdateSpaceStatus :exec
-- ======================================================================
UPDATE spaces SET status = $1, updated_at = NOW() WHERE id = $2;

-- ======================================================================
-- name: AddSpaceParticipant :exec
-- ======================================================================
INSERT INTO space_participants (space_id, user_id, role, status, joined_at, is_host)
VALUES ($1, $2, $3, $4, NOW(), $5);

-- ======================================================================
-- name: RemoveSpaceParticipant :exec
-- ======================================================================
UPDATE space_participants SET left_at = NOW() 
WHERE space_id = $1 AND user_id = $2;

-- ======================================================================
-- name: GetSpaceParticipants :many
-- ======================================================================
SELECT sp.*, u.username, u.full_name, u.avatar_url
FROM space_participants sp
JOIN users u ON sp.user_id = u.id
WHERE sp.space_id = $1 AND sp.left_at IS NULL
ORDER BY sp.joined_at ASC;

-- ======================================================================
-- name: IsUserInSpace :one
-- ======================================================================
SELECT EXISTS(SELECT 1 FROM space_participants 
WHERE space_id = $1 AND user_id = $2 AND left_at IS NULL);

-- ======================================================================
-- name: GetDailyStats :many
-- ======================================================================
SELECT 
    DATE(created_at) AS date,
    COUNT(*) AS total,
    COUNT(DISTINCT user_id) AS unique_users
FROM $1
WHERE created_at >= $2 AND created_at <= $3
GROUP BY DATE(created_at)
ORDER BY date ASC;

-- ======================================================================
-- name: GetSystemSettings :one
-- ======================================================================
SELECT * FROM system_settings LIMIT 1;

-- ======================================================================
-- name: UpdateSystemSettings :exec
-- ======================================================================
UPDATE system_settings SET
    site_name = $1,
    site_description = $2,
    max_tweet_length = $3,
    max_media_count = $4,
    max_image_size_mb = $5,
    max_video_size_mb = $6,
    allow_registration = $7,
    require_email_verification = $8,
    default_language = $9,
    default_theme = $10,
    maintenance_mode = $11,
    maintenance_message = $12,
    updated_at = NOW()
WHERE id = $13;