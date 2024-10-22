// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"testing"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func Test_parseFlags(t *testing.T) {
	flags := Flags{}
	os.Args = []string{"test", "--metrics-bind-address", "8081", "--server", "foo", "--cache-freq", "10s", "--per-user-metrics", "true"}
	parseFlags(&flags)
	if flags.metricsAddr != "8081" {
		t.Errorf("Test_parseFlags() metricsAddr = %v, want %v", flags.metricsAddr, "8081")
	}
	if flags.server != "foo" {
		t.Errorf("Test_parseFlags() server = %v, want %v", flags.server, "foo")
	}
	if flags.cacheFreq != time.Second*10 {
		t.Errorf("Test_parseFlags() cacheFreq = %v, want %v", flags.cacheFreq, time.Second*10)
	}
	if !flags.perUserMetrics {
		t.Errorf("Test_parseFlags() perUserMetrics = %v, want %v", flags.perUserMetrics, false)
	}
}
