// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/types"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/SlinkyProject/slurm-exporter/internal/exporter"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

// Input flags to the command
type Flags struct {
	metricsAddr    string
	server         string
	cacheFreq      time.Duration
	perUserMetrics bool
}

func parseFlags(flags *Flags) {
	flag.StringVar(
		&flags.metricsAddr,
		"metrics-bind-address",
		":8080",
		"The address the metric endpoint binds to.",
	)
	flag.StringVar(
		&flags.server,
		"server",
		"http://slurm-restapi:6820",
		"The server url of the cluster for the exporter to monitor.",
	)
	flag.DurationVar(
		&flags.cacheFreq,
		"cache-freq",
		5*time.Second,
		"The amount of time to wait between updating the slurm restapi cache. Must be greater than 1s and must be parsable by time.ParseDuration.",
	)
	flag.BoolVar(
		&flags.perUserMetrics,
		"per-user-metrics",
		false,
		"Enable per-user metrics data. Enabling this could significantly increase prometheus storage requirements.",
	)
	flag.Parse()
}

func main() {
	var flags Flags
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	parseFlags(&flags)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if flags.cacheFreq <= 1*time.Second {
		setupLog.Error(errors.New("config"), "Must use a cache-freq > 1s.")
		os.Exit(1)
	}

	name := types.NamespacedName{
		Namespace: "",
		Name:      "exporter",
	}
	slurmCollector := exporter.NewSlurmCollector(name, flags.server, flags.cacheFreq, flags.perUserMetrics)
	if err := slurmCollector.SlurmClient(); err != nil {
		setupLog.Error(err, "could not start slurm client")
		os.Exit(1)
	}
	prometheus.MustRegister(slurmCollector)

	setupLog.Info("starting exporter")
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(flags.metricsAddr, nil); err != nil {
		setupLog.Error(err, "problem running exporter")
		os.Exit(1)
	}
}
