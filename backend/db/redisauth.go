package db

import (
	"context"
	"hexchess-lib/logutil"
	"hexchess-lib/timeutil"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	redigo "github.com/gomodule/redigo/redis"
)

const (
	// "The IAM authentication token is valid for 15 minutes"
	// https://docs.aws.amazon.com/memorydb/latest/devguide/auth-iam.html#auth-iam-limits
	tokenValiditySeconds = 900

	connectAction       = "connect"
	awsCacheServiceName = "memorydb" // needs to match whatever the database on AWS is.

	// if the request has no payload, you should use the hex-encoded SHA-256 of an empty string as the payloadHash value.
	hexEncodedSHA256EmptyString = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	redisTokenRefreshPeriod = 10 * time.Minute
)

type RedisTokenRefresher struct {
	redisUsername  string
	awsRegion      string
	tokenRequest   *http.Request
	awsCredentials aws.Credentials
	signer         *v4.Signer

	token atomic.Pointer[string]

	cancel func()
}

func NewRedisTokenRefresher(ctx context.Context, region, redisUsername, clusterName string) *RedisTokenRefresher {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		logutil.Fatal("load aws config", err)
	}
	credentials, err := awsCfg.Credentials.Retrieve(ctx)
	if err != nil {
		logutil.Fatal("retrieve aws credentials", err)
	}

	queryParams := url.Values{
		"Action":        {connectAction},
		"User":          {redisUsername},
		"X-Amz-Expires": {strconv.FormatInt(int64(tokenValiditySeconds), 10)},
	}
	authURL := url.URL{Host: clusterName, Scheme: "http", Path: "/", RawQuery: queryParams.Encode()}
	request, err := http.NewRequest(http.MethodGet, authURL.String(), nil)
	if err != nil {
		logutil.Fatal("create presigned http request", err)
	}

	refresher := &RedisTokenRefresher{
		redisUsername:  redisUsername,
		awsRegion:      region,
		tokenRequest:   request,
		awsCredentials: credentials,
		signer:         v4.NewSigner(),
	}

	refresher.acquireToken()
	refresher.cancel = timeutil.Schedule(redisTokenRefreshPeriod, refresher.acquireToken)

	return refresher
}

func (refresh *RedisTokenRefresher) acquireToken() {
	signedURL, _, err := refresh.signer.PresignHTTP(
		context.Background(),
		refresh.awsCredentials,
		refresh.tokenRequest,
		hexEncodedSHA256EmptyString,
		awsCacheServiceName,
		refresh.awsRegion,
		time.Now().UTC(),
	)
	if err != nil {
		slog.Error("failed to generate presigned url for redis auth", "error", err, "redisUsername", refresh.redisUsername)
		return
	}
	signedURL = strings.Replace(signedURL, "http://", "", 1)
	refresh.token.Store(&signedURL)
}

func (refresh *RedisTokenRefresher) Shutdown() {
	if refresh.cancel != nil {
		refresh.cancel()
	}
}

func NewCredentialsProvider(refresh *RedisTokenRefresher) func() (username string, password string) {
	return func() (username string, password string) {
		token := refresh.token.Load()
		if token != nil {
			password = *token
		} else {
			slog.Warn("redis credentials provider: token not available", "redisUsername", refresh.redisUsername)
		}
		return refresh.redisUsername, password
	}
}

func NewSecureDialer(refresh *RedisTokenRefresher, addr string) func() (redigo.Conn, error) {
	return func() (redigo.Conn, error) {
		token := refresh.token.Load()
		var password string
		if token != nil {
			password = *token
		} else {
			slog.Warn("redis dial: token not available", "redisUsername", refresh.redisUsername)
		}
		return redigo.Dial("tcp", addr, redigo.DialUsername(refresh.redisUsername), redigo.DialPassword(password))
	}
}
