package redisq

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client
var Ctx = context.Background()

const QueueName = "discovery_tasks"
const UserQueuePrefix = QueueName + ":user:"

func QueueNameForUser(userID uint) string {
	if userID == 0 {
		return QueueName
	}
	return fmt.Sprintf("%s%d", UserQueuePrefix, userID)
}

func UserQueuePattern() string {
	return UserQueuePrefix + "*"
}

func UserIDFromQueueName(key string) (uint, bool) {
	if !strings.HasPrefix(key, UserQueuePrefix) {
		return 0, false
	}
	raw := strings.TrimPrefix(key, UserQueuePrefix)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

func UserQueueKeys(ctx context.Context) ([]string, error) {
	keys, err := Client.Keys(ctx, UserQueuePattern()).Result()
	if err != nil {
		return nil, err
	}
	sort.Slice(keys, func(i, j int) bool {
		ai, aok := UserIDFromQueueName(keys[i])
		bi, bok := UserIDFromQueueName(keys[j])
		if aok && bok {
			return ai < bi
		}
		return keys[i] < keys[j]
	})
	return keys, nil
}

func Connect() {
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	addr := fmt.Sprintf("%s:%s", redisHost, redisPort)

	log.Println("Connecting to Redis...")
	Client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	_, err := Client.Ping(Ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis! \n", err)
	}
	log.Println("Connected to Redis successfully!")
}
