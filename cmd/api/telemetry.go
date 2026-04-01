package main

import (
	"context"

	"github.com/holmes89/archaea/telemetry"
	"go.uber.org/zap"
)

func initOTel(ctx context.Context, serviceName string, logger *zap.Logger) (func(context.Context), error) {
	return telemetry.Init(ctx, serviceName, logger)
}
