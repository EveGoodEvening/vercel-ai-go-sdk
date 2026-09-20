package gateway

import (
	"context"
	"math"
	"time"
)

func (client *Client) retryDelay(retryNumber int, responseErr *ResponseError) time.Duration {
	policy := client.config.retryPolicy
	if retryAfter, ok := responseErr.RetryAfter(); ok {
		if retryAfter > policy.MaxDelay {
			return policy.MaxDelay
		}
		return retryAfter
	}

	delayValue := float64(policy.InitialDelay)
	maxDelayValue := float64(policy.MaxDelay)
	for retry := 1; retry < retryNumber; retry++ {
		if delayValue >= maxDelayValue/policy.Multiplier {
			delayValue = maxDelayValue
			break
		}
		delayValue *= policy.Multiplier
	}
	delay := time.Duration(math.Round(delayValue))
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	if policy.Jitter != 0 {
		u := client.config.retryHooks.jitter()
		delay = time.Duration(math.Round(float64(delay) * (1 + policy.Jitter*(2*u-1))))
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
	}
	return delay
}

func (client *Client) waitForRetry(ctx context.Context, retryNumber int, responseErr *ResponseError) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := client.config.retryHooks.sleep(ctx, client.retryDelay(retryNumber, responseErr)); err != nil {
		return err
	}
	return ctx.Err()
}

func (client *Client) canRetry(ctx context.Context, attempt int, responseErr *ResponseError) bool {
	return attempt < client.config.retryPolicy.MaxAttempts &&
		ctx.Err() == nil &&
		responseErr != nil &&
		responseErr.cause == nil &&
		responseErr.Retryable()
}
