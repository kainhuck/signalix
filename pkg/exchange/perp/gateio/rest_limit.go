package gateio

import "context"

func (c *Client) waitREST(ctx context.Context) error {
	if c == nil || c.restLimiter == nil {
		return nil
	}
	return c.restLimiter.Wait(ctx)
}
