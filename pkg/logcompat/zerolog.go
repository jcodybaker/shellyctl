package logcompat

import (
	golog "log"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"k8s.io/klog/v2"
)

type LogWriter struct {
	log *zerolog.Logger
}

func Init(l *zerolog.Logger) {
	// Both the mDNS and mgRPC libraries log directly to klog and go log, which is annoying.
	mGRPCLogger := log.Logger.With().Str("component", "mgrpc").Logger()
	klog.SetLogger(zerologr.New(&mGRPCLogger))
	golog.SetOutput(&LogWriter{log: l})
}
