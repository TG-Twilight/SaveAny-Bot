package bot

import (
	"context"
	"net/url"
	"os"
	"time"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"
	"github.com/krau/SaveAny-Bot/config"
	"github.com/krau/SaveAny-Bot/logger"
	"golang.org/x/net/proxy"
)

var Client *gotgproto.Client

func newProxyDialer(proxyUrl string) (proxy.Dialer, error) {
	url, err := url.Parse(proxyUrl)
	if err != nil {
		return nil, err
	}
	return proxy.FromURL(url, proxy.Direct)
}

func Init() {
	logger.L.Info("Initializing client...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resultChan := make(chan struct {
		client *gotgproto.Client
		err    error
	})
	go func() {
		var resolver dcs.Resolver
		if config.Cfg.Telegram.Proxy.Enable && config.Cfg.Telegram.Proxy.URL != "" {
			dialer, err := newProxyDialer(config.Cfg.Telegram.Proxy.URL)
			if err != nil {
				resultChan <- struct {
					client *gotgproto.Client
					err    error
				}{nil, err}
				return
			}
			resolver = dcs.Plain(dcs.PlainOptions{
				Dial: dialer.(proxy.ContextDialer).DialContext,
			})
		} else {
			resolver = dcs.DefaultResolver()
		}
		client, err := gotgproto.NewClient(config.Cfg.Telegram.AppID,
			config.Cfg.Telegram.AppHash,
			gotgproto.ClientTypeBot(config.Cfg.Telegram.Token),
			&gotgproto.ClientOpts{
				Session:          sessionMaker.SqlSession(sqlite.Open("data/session.db")),
				DisableCopyright: true,
				Middlewares:      FloodWaitMiddleware(),
				Resolver:         resolver,
			},
		)
		if err != nil {
			resultChan <- struct {
				client *gotgproto.Client
				err    error
			}{nil, err}
			return
		}
		_, err = client.API().BotsSetBotCommands(ctx, &tg.BotsSetBotCommandsRequest{
			Scope: &tg.BotCommandScopeDefault{},
			Commands: []tg.BotCommand{
				{Command: "start", Description: "开始使用"},
				{Command: "help", Description: "显示帮助"},
				{Command: "silent", Description: "开启/关闭静默模式"},
				{Command: "storage", Description: "设置默认存储端"},
				{Command: "save", Description: "保存所回复的文件"},
				{Command: "path", Description: "更改保存路径配置"},
			},
		})
		resultChan <- struct {
			client *gotgproto.Client
			err    error
		}{client, err}
	}()

	select {
	case <-ctx.Done():
		logger.L.Fatal("Failed to initialize client: timeout")
		os.Exit(1)
	case result := <-resultChan:
		if result.err != nil {
			logger.L.Fatalf("Failed to initialize client: %s", result.err)
			os.Exit(1)
		}
		Client = result.client
		RegisterHandlers(Client.Dispatcher)
		logger.L.Info("Client initialized")
	}
}
