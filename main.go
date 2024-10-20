// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

// package main is the entry point for kratos.
package main

import (
	"context"
	"log"

	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	"github.com/ory/kratos/driver"
	"github.com/ory/kratos/cmd"
	"github.com/ory/x/configx"
	"github.com/ory/x/dbal"
	"github.com/ory/x/servicelocatorx"
)

func main() {

	dbal.RegisterDriver(func() dbal.Driver {
		return driver.NewRegistryDefault()
	})

	ctx := context.Background()
	opts := configx.ConfigOptionsFromContext(ctx)
	slOpts := []servicelocatorx.Option(nil)
	sl := servicelocatorx.NewOptions(slOpts...)

	flags := pflag.NewFlagSet("myflags", pflag.ExitOnError)
	flags.Bool("dev", true, "Enable development mode")
	flags.Bool("watch-courier", true, "Enable watch courier")
	flags.StringSlice("config", []string{"D:\\Dev\\kratos/contrib/quickstart/kratos/email-password/kratos.yml"}, "Path to the configuration file")

	d, err := driver.New(ctx, nil, sl, nil, append(opts, configx.WithFlags(flags)))


	
	if err != nil {
		log.Fatal(err)
	}

	g, _ := errgroup.WithContext(ctx)
	cmd.ServePublic(d, nil, g, sl, nil)
	cmd.ServeAdmin(d, nil, g, sl, nil)

	g.Go(func() error {
		return cmd.BgTasks(d, nil, nil)
	})

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
	
}

//creat emain.go fucntion
