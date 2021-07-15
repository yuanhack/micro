package rtss_config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gitee.com/smartsteps/go-micro-plugins/store/mongo"
	"gitee.com/smartsteps/go-micro/v2"
	"gitee.com/smartsteps/go-micro/v2/config/cmd"
	"github.com/micro/cli/v2"
	"github.com/patrickmn/go-cache"

	//proto "gitee.com/smartsteps/go-micro/v2/config/source/service/proto"
	log "gitee.com/smartsteps/go-micro/v2/logger"
	"gitee.com/smartsteps/go-micro/v2/store"
	"github.com/micro/micro/v2/internal/client"
	"github.com/micro/micro/v2/internal/helper"
	"github.com/micro/micro/v2/service/rtss_config/handler"
	proto "github.com/micro/micro/v2/service/rtss_config/proto"
)

var (
	// Service name
	Name = "go.micro.rtss_config"
	// Default database store
	Database = "store"
	// Default key
	Namespace = "global"

	MgoUrl          string
	MgoDatabaseName string
	MgoTableName    string

	CacheDuration        time.Duration
	CacheCleanupInterval time.Duration
)

func Run(c *cli.Context, srvOpts ...micro.Option) {
	if len(c.String("server_name")) > 0 {
		Name = c.String("server_name")
	}

	if len(c.String("watch_topic")) > 0 {
		handler.WatchTopic = c.String("watch_topic")
	}

	srvOpts = append(srvOpts, micro.Name(Name))

	service := micro.NewService(srvOpts...)

	h := &handler.Config{
		Store: *cmd.DefaultCmd.Options().Store,
		Cache: cache.New(CacheDuration, CacheCleanupInterval),
	}
	opts := []store.Option{
		mongo.URI(MgoUrl),
		store.Database(MgoDatabaseName),
		store.Table(MgoTableName),
	}
	h.Store = mongo.NewStore(opts...)

	proto.RegisterConfigHandler(service.Server(), h)
	micro.RegisterSubscriber(handler.WatchTopic, service.Server(), handler.Watcher)

	log.Infof("uri %v", MgoUrl)
	log.Infof("dbname %v", MgoDatabaseName)
	log.Infof("table %v", MgoTableName)

	if err := service.Run(); err != nil {
		log.Fatalf("config Run the service error: ", err)
	}
}

func setConfig(ctx *cli.Context) error {
	pb := proto.NewConfigService("go.micro.rtss_config", client.New(ctx))

	args := ctx.Args()

	if args.Len() == 0 {
		fmt.Println("Required usage: micro config set key val")
		os.Exit(1)
	}

	// key val
	key := args.Get(0)
	val := args.Get(1)

	// TODO: allow the specifying of a config.Key. This will be service name
	// The actuall key-val set is a path e.g micro/accounts/key
	_, err := pb.Update(context.TODO(), &proto.UpdateRequest{
		Change: &proto.Change{
			// global key
			Namespace: Namespace,
			// actual key for the value
			Path: key,
			// The value
			ChangeSet: &proto.ChangeSet{
				Data:      string(val),
				Format:    "json",
				Source:    "cli",
				Timestamp: time.Now().Unix(),
			},
		},
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return nil
}

func getConfig(ctx *cli.Context) error {
	pb := proto.NewConfigService("go.micro.rtss_config", client.New(ctx))

	args := ctx.Args()

	if args.Len() == 0 {
		fmt.Println("Required usage: micro config get key")
		os.Exit(1)
	}

	// key val
	key := args.Get(0)

	if len(key) == 0 {
		log.Fatal("key cannot be blank")
	}

	// TODO: allow the specifying of a config.Key. This will be service name
	// The actuall key-val set is a path e.g micro/accounts/key

	rsp, err := pb.Read(context.TODO(), &proto.ReadRequest{
		// The global key,
		Namespace: Namespace,
		// The actual key for the val
		Path: key,
	})

	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			fmt.Println("not found")
			os.Exit(1)
		}
		fmt.Println(err)
		os.Exit(1)
	}

	if rsp.Change == nil || rsp.Change.ChangeSet == nil {
		fmt.Println("not found")
		os.Exit(1)
	}

	// don't do it
	if v := rsp.Change.ChangeSet.Data; len(v) == 0 || string(v) == "null" {
		fmt.Println("not found")
		os.Exit(1)
	}

	fmt.Println(string(rsp.Change.ChangeSet.Data))

	return nil
}

func delConfig(ctx *cli.Context) error {
	pb := proto.NewConfigService("go.micro.rtss_config", client.New(ctx))

	args := ctx.Args()

	if args.Len() == 0 {
		fmt.Println("Required usage: micro config get key")
		os.Exit(1)
	}

	// key val
	key := args.Get(0)

	if len(key) == 0 {
		log.Fatal("key cannot be blank")
	}

	// TODO: allow the specifying of a config.Key. This will be service name
	// The actuall key-val set is a path e.g micro/accounts/key

	_, err := pb.Delete(context.TODO(), &proto.DeleteRequest{
		Change: &proto.Change{
			// The global key,
			Namespace: Namespace,
			// The actual key for the val
			Path: key,
		},
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return nil
}

func Commands(options ...micro.Option) []*cli.Command {
	command := &cli.Command{
		Name:  "rtss_config",
		Usage: "Manage configuration values",
		Subcommands: []*cli.Command{
			{
				Name:   "set",
				Usage:  "Set a key-val; micro config set key val",
				Action: setConfig,
			},
			{
				Name:   "get",
				Usage:  "Get a value; micro config get key",
				Action: getConfig,
			},
			{
				Name:   "del",
				Usage:  "Delete a value; micro config del key",
				Action: delConfig,
			},
		},
		Action: func(ctx *cli.Context) error {
			if err := helper.UnexpectedSubcommand(ctx); err != nil {
				return err
			}
			Run(ctx, options...)
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "namespace",
				EnvVars: []string{"MICRO_CONFIG_NAMESPACE"},
				Usage:   "Set the namespace used by the Config Service e.g. go.micro.rtss_config",
			},
			&cli.StringFlag{
				Name:    "watch_topic",
				EnvVars: []string{"MICRO_CONFIG_WATCH_TOPIC"},
				Usage:   "watch the change event.",
			},

			&cli.DurationFlag{
				Name:        "cache_duration",
				EnvVars:     []string{"MICRO_CONFIG_CACHE_DURATION"},
				Usage:       "set cache duration.",
				Value:       time.Minute * 5,
				Destination: &CacheDuration,
			},
			&cli.DurationFlag{
				Name:        "cache_cleanup_interval",
				EnvVars:     []string{"MICRO_CONFIG_CACHE_CLEANUP_INTERVAL"},
				Usage:       "set cache cleanup interval.",
				Value:       time.Minute * 2,
				Destination: &CacheCleanupInterval,
			},

			&cli.StringFlag{
				Name:        "mgo_url",
				EnvVars:     []string{"MICRO_CONFIG_MGO_URL"},
				Usage:       "set mongodb url.",
				Required:    true,
				Destination: &MgoUrl,
			},
			&cli.StringFlag{
				Name:        "mgo_database",
				EnvVars:     []string{"MICRO_CONFIG_MGO_DATABASE"},
				Usage:       "set mongodb database name.",
				Value:       "rtss_config",
				Destination: &MgoDatabaseName,
			},
			&cli.StringFlag{
				Name:        "mgo_table",
				EnvVars:     []string{"MICRO_CONFIG_MGO_TABLE"},
				Usage:       "set mongodb table name.",
				Value:       "micro_store",
				Destination: &MgoTableName,
			},
		},
	}

	for _, p := range Plugins() {
		if cmds := p.Commands(); len(cmds) > 0 {
			command.Subcommands = append(command.Subcommands, cmds...)
		}

		if flags := p.Flags(); len(flags) > 0 {
			command.Flags = append(command.Flags, flags...)
		}
	}

	return []*cli.Command{command}
}
