package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/config"
	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway/cashinpay"
	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway/generic"
	"github.com/matspectrum-ai/Claro-Fatura/internal/httpapi"
	"github.com/matspectrum-ai/Claro-Fatura/internal/invoice"
	"github.com/matspectrum-ai/Claro-Fatura/internal/payment"
	"github.com/matspectrum-ai/Claro-Fatura/internal/supabase"
)

func main() {
	logger:=slog.New(slog.NewJSONHandler(os.Stdout,nil))
	cfg,err:=config.Load();if err!=nil{logger.Error("configuration error","error",err);os.Exit(1)}
	store:=supabase.New(cfg.SupabaseURL,cfg.SupabaseServiceRoleKey)
	fallback:=generic.New()
	registry:=gateway.NewRegistry(fallback,cashinpay.New(cfg.CashinPaySecretKey,cfg.CashinPayWebhookSecret,cfg.ProductName,gateway.DefaultCustomerEmail))
	creator:=payment.NewWithExpiration(store,registry,cfg.ProductName,cfg.PIXExpiration)
	generator:=payment.NewGenerator(store,creator)
	confirmer:=payment.NewConfirmer(store,registry)
	status:=payment.NewStatusService(store,confirmer)
	webhooks:=payment.NewWebhookService(confirmer)
	invoices:=invoice.New(store)

	handler:=httpapi.New(httpapi.Dependencies{Invoices:invoices,PIX:generator,Status:status,Webhooks:webhooks,SiteURL:cfg.SiteURL,LightDir:"Light"},logger)
	server:=&http.Server{Addr:cfg.Addr,Handler:handler,ReadHeaderTimeout:5*time.Second,ReadTimeout:15*time.Second,WriteTimeout:35*time.Second,IdleTimeout:90*time.Second}

	done:=make(chan os.Signal,1);signal.Notify(done,syscall.SIGINT,syscall.SIGTERM)
	go func(){logger.Info("server listening","addr",cfg.Addr);if err:=server.ListenAndServe();err!=nil&&err!=http.ErrServerClosed{logger.Error("server failed","error",err);os.Exit(1)}}()
	<-done
	ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);defer cancel()
	if err:=server.Shutdown(ctx);err!=nil{logger.Error("shutdown failed","error",err);os.Exit(1)}
	logger.Info("shutdown complete")
}
