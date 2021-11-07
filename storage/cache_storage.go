package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"microblogging-service/data"
	"time"
)

type CacheStorage struct {
	persistence Storage
	client 		*redis.Client
}

const expirationTime = 1 * time.Hour

func (c *CacheStorage) setPost(ctx context.Context, post *data.Post) error {
	rawData, err := json.Marshal(post)
	if err != nil {
		return fmt.Errorf("failed to marshall json due to an error - %w", err)
	}
	postIdKey := c.postIdKey(post.Id)
	if err = c.client.Set(ctx, postIdKey, rawData, expirationTime).Err(); err != nil {
		return fmt.Errorf("failed to insert key %s into cache due to an error - %w", postIdKey, err)
	}
	return nil
}

func (c *CacheStorage) AddPost(ctx context.Context, post *data.Post) error {
	if err := c.persistence.AddPost(ctx, post); err != nil {
		return err
	}
	if err := c.setPost(ctx, post); err != nil {
		return fmt.Errorf("%w: %s", ErrBase, err)
	}
	return nil
}

func (c *CacheStorage) GetPost(ctx context.Context, postId data.PostId) (*data.Post, error) {
	postIdKey := c.postIdKey(postId)
	result := c.client.Get(ctx, postIdKey)
	switch rawData, err := result.Result(); {
	case err == redis.Nil:
	// continue execution
	case err != nil:
		return nil, fmt.Errorf("%w: failed to get value from cache due to error - %s", ErrBase, err)
	default:
		log.Printf("Successfully obtained data from cache for key %s", postId)
		var post data.Post
		if err = json.Unmarshal([]byte(rawData), &post); err != nil {
			return nil, fmt.Errorf("%w: failed to unmarshall json due to an error - %s", ErrBase, err)
		}
		return &post, nil
	}
	log.Printf("Loading post %s from persistent storage", postId)
	post, err := c.persistence.GetPost(ctx, postId)
	if err != nil {
		return nil, err
	}
	if err = c.setPost(ctx, post); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBase, err)
	}
	return post, nil
}

func (c *CacheStorage) GetUserPosts (ctx context.Context, userId data.UserId,
	token data.PageToken, limit int) ([]*data.Post, data.PageToken, error) {

	return c.persistence.GetUserPosts(ctx, userId, token, limit)
}

func (c *CacheStorage) UpdatePost(ctx context.Context, post *data.Post) error {
	if err := c.persistence.UpdatePost(ctx, post); err != nil {
		return err
	}
	if err := c.setPost(ctx, post); err != nil {
		return fmt.Errorf("%w: %s", ErrBase, err)
	}
	return nil
}

func NewCacheStorage(persistentStorage Storage, redisUrl string) *CacheStorage {
	return &CacheStorage{
		persistence: persistentStorage,
		client: redis.NewClient(&redis.Options{Addr: redisUrl}),
	}
}

func (c *CacheStorage) postIdKey(id data.PostId) string {
	return "post:" + string(id)
}
