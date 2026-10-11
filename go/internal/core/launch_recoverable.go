package core

import "context"

type LaunchFunc func(context.Context, BridgeRequest) (BridgeResponse, error)

func LaunchRecoverable(ctx context.Context, launch LaunchFunc, req BridgeRequest, onRecoverable func(BridgeResponse) BridgeRequest) (BridgeResponse, error) {
	resp, err := launch(ctx, req)
	if !resp.SessionRecoverable || ctx.Err() != nil {
		return resp, err
	}
	fresh := onRecoverable(resp)
	fresh.FreshSession = true
	resp, err = launch(ctx, fresh)
	resp.SessionRecoverable = false
	return resp, err
}

func SameRequest(req BridgeRequest) func(BridgeResponse) BridgeRequest {
	return func(BridgeResponse) BridgeRequest { return req }
}

func LaunchWithFreshSession(ctx context.Context, b Bridge, req BridgeRequest) (BridgeResponse, error) {
	return LaunchRecoverable(ctx, b.Launch, req, SameRequest(req))
}
