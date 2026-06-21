package db

import (
	"context"
	"hexchess-svc/lib/logutil"
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
)

const (
	// "The IAM authentication token is valid for 15 minutes"
	// https://docs.aws.amazon.com/memorydb/latest/devguide/auth-iam.html#auth-iam-limits
	tokenValiditySeconds = 900

	connectAction = "connect"

	// If the request has no payload you should use the hex encoded SHA-256 of an empty string as the payloadHash value.
	hexEncodedSHA256EmptyString = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	redisTokenRefreshPeriod = 10 * time.Minute
)

type RedisConnector struct {
	serviceName string
	username    string
	region      string
	req         *http.Request
	credentials aws.Credentials
	signer      *v4.Signer
	token       atomic.Value
	ticker      *time.Ticker
}

func NewRedisConnector(ctx context.Context, region, username, clusterName string) *RedisConnector {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		logutil.Fatal("load aws config", err)
	}

	credentials, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		logutil.Fatal("retrieve aws credentials", err)
	}
	if credentials.AccessKeyID == "" || credentials.SecretAccessKey == "" {
		logutil.Fatal("aws credentials are empty", nil)
	}

	queryParams := url.Values{
		"Action":        {connectAction},
		"User":          {username},
		"X-Amz-Expires": {strconv.FormatInt(int64(tokenValiditySeconds), 10)},
	}
	authURL := url.URL{
		Host:     clusterName,
		Scheme:   "http",
		Path:     "/",
		RawQuery: queryParams.Encode(),
	}
	req, err := http.NewRequest(http.MethodGet, authURL.String(), nil)
	if err != nil {
		logutil.Fatal("create presigned http request struct", err)
	}

	connector := &RedisConnector{
		serviceName: clusterName,
		region:      region,
		req:         req,
		credentials: credentials,
		signer:      v4.NewSigner(),
		ticker:      time.NewTicker(redisTokenRefreshPeriod),
	}
	go connector.refreshLoop()
	return connector
}

func (c *RedisConnector) Stop() {
	c.ticker.Stop()
}

func (c *RedisConnector) refreshLoop() {
	for range c.ticker.C {
		signedURL, _, err := c.signer.PresignHTTP(
			context.Background(),
			c.credentials,
			c.req,
			hexEncodedSHA256EmptyString,
			c.serviceName,
			c.region,
			time.Now().UTC(),
		)
		if err != nil {
			slog.Error("failed to generate presigned url for redis connection", "error", err)
			continue
		}
		signedURL = strings.Replace(signedURL, "http://", "", 1)
		c.token.Store(authToken{value: signedURL, issuedAt: time.Now()})
	}
}

func (c *RedisConnector) CredentialsProvider() (username string, password string) {
	token := c.token.Load().(authToken)
	slog.Info("redis credentials provider", "serviceName", c.serviceName, "tokenIssuedAt", token.issuedAt, "tokenLifetime", time.Since(token.issuedAt))
	return c.username, token.value
}
