package klog

import (
	"context"
	"flag"

	"k8s.io/klog/v2"
)

func InitFlag(flagset *flag.FlagSet) {
	klog.InitFlags(flagset)
}

func NewContext(name string) context.Context {
	return context.WithValue(context.Background(), "name", name)
}

func Infof(ctx context.Context, format string, args ...interface{}) {
	name := getName(ctx)
	klog.Infof("%s: "+format, append([]interface{}{name}, args...))
}

func getName(ctx context.Context) string {
	n, ok := ctx.Value("name").(string)
	if !ok {
		return ""
	}

	return n
}
