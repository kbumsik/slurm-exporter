// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	"github.com/SlinkyProject/slurm-client/pkg/client"
	"github.com/SlinkyProject/slurm-client/pkg/fake"
	"github.com/SlinkyProject/slurm-client/pkg/interceptor"
	"github.com/SlinkyProject/slurm-client/pkg/object"

	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"
)

var (
	exporter_url = "http://localhost:8080"
	name         = types.NamespacedName{
		Namespace: "default",
		Name:      "exporter_test",
	}
	cacheFreq = 5 * time.Second
)

// newSlurmCollectorHelper will initialize a Slurm Collector for
// metrics and will also load a predefined set of objects into
// the cache of a fake Slurm client.
func newSlurmCollectorHelper() SlurmCollector {
	sc := NewSlurmCollector(name, exporter_url, cacheFreq, true)

	jobs := &slurmtypes.JobInfoList{}
	partitions := &slurmtypes.PartitionInfoList{}
	nodes := &slurmtypes.NodeList{}

	jobs.AppendItem(jobInfoA)
	jobs.AppendItem(jobInfoB)
	jobs.AppendItem(jobInfoC)
	partitions.AppendItem(partitionA)
	partitions.AppendItem(partitionB)
	nodes.AppendItem(nodeA)
	nodes.AppendItem(nodeB)
	nodes.AppendItem(nodeC)

	client := newFakeClientList(interceptor.Funcs{}, jobs, partitions, nodes)
	sc.slurmClusters.Add(name, client)
	return *sc
}

// newFakeClientList will return a fake Slurm client with interceptor functions
// and lists of Slurm objects.
func newFakeClientList(interceptorFuncs interceptor.Funcs, initObjLists ...object.ObjectList) client.Client {
	return fake.NewClientBuilder().
		WithLists(initObjLists...).
		WithInterceptorFuncs(interceptorFuncs).
		Build()
}

// TestSlurmParse will test that slurmParse() correctly calculates the
// metrics for a predefined cache of Slurm objects.
func TestSlurmParse(t *testing.T) {
	os.Setenv("METRICS_TOKEN", "foo")
	sc := NewSlurmCollector(name, exporter_url, cacheFreq, false)

	jobs := &slurmtypes.JobInfoList{}
	partitions := &slurmtypes.PartitionInfoList{}
	nodes := &slurmtypes.NodeList{}

	jobs.AppendItem(jobInfoA)
	jobs.AppendItem(jobInfoB)
	jobs.AppendItem(jobInfoC)
	jobs.AppendItem(jobInfoD)
	partitions.AppendItem(partitionA)
	partitions.AppendItem(partitionB)
	nodes.AppendItem(nodeA)
	nodes.AppendItem(nodeB)
	nodes.AppendItem(nodeC)

	slurmData := sc.slurmParse(jobs, nodes, partitions)
	assert.NotEmpty(t, slurmData)

	assert.Equal(t, 2, len(slurmData.jobs))
	assert.Equal(t, &JobData{Count: 2, Pending: 1, Running: 1, Hold: 1}, slurmData.jobs["slurm"])
	assert.Equal(t, &JobData{Count: 1, Pending: 1, Running: 0, Hold: 0}, slurmData.jobs["hal"])
	assert.Equal(t, 2, len(slurmData.partitions))
	assert.Equal(t, &PartitionData{Nodes: 2, Cpus: 20, PendingJobs: 1, PendingMaxNodes: 1, Jobs: 1, RunningJobs: 0, HoldJobs: 0, Alloc: 4, Idle: 16}, slurmData.partitions["purple"])
	assert.Equal(t, &PartitionData{Nodes: 2, Cpus: 32, PendingJobs: 1, PendingMaxNodes: 0, Jobs: 2, RunningJobs: 1, HoldJobs: 1, Alloc: 16, Idle: 16}, slurmData.partitions["green"])
	assert.Equal(t, len(slurmData.nodes), 3)
	assert.Equal(t, &NodeData{Cpus: 12, Alloc: 0, Idle: 12}, slurmData.nodes["kind-worker"])
	assert.Equal(t, &NodeData{Cpus: 24, Alloc: 12, Idle: 12}, slurmData.nodes["kind-worker2"])
	assert.Equal(t, &NodeData{Cpus: 8, Alloc: 4, Idle: 4}, slurmData.nodes["kind-worker3"])
	assert.Equal(t, NodeStates{allocated: 1, completing: 1, down: 1, drain: 1, err: 1, idle: 1, maintenance: 1, mixed: 1, reserved: 1}, slurmData.nodestates)
}

// TestSlurmClient will test how SlurmClient() initializes
// a Slurm client.
func TestSlurmClient(t *testing.T) {
	os.Unsetenv("METRICS_TOKEN")
	sc := NewSlurmCollector(name, exporter_url, cacheFreq, false)
	err := sc.SlurmClient()
	assert.NotNil(t, err)

	os.Setenv("METRICS_TOKEN", "")
	sc = NewSlurmCollector(name, exporter_url, cacheFreq, false)
	err = sc.SlurmClient()
	assert.NotNil(t, err)

	os.Setenv("METRICS_TOKEN", "foo")
	sc = NewSlurmCollector(name, "", cacheFreq, false)
	err = sc.SlurmClient()
	assert.NotNil(t, err)

	sc = NewSlurmCollector(name, exporter_url, cacheFreq, false)
	err = sc.SlurmClient()
	assert.Equal(t, nil, err)
	assert.NotNil(t, sc.slurmClusters.Get(name))
}

// TestSlurmCollectType will test that slurmCollectType returns
// the correct data from a Slurm client. A fake Slurm
// client is used and preloaded with a cache of objects. In order
// to simulate error conditions, interceptor functions are used.
func TestSlurmCollectType(t *testing.T) {

	sc := newSlurmCollectorHelper()

	jobsTest := &slurmtypes.JobInfoList{}
	partitionsTest := &slurmtypes.PartitionInfoList{}
	nodesTest := &slurmtypes.NodeList{}

	err := sc.slurmCollectType(jobsTest)
	assert.Equal(t, nil, err)
	assert.Equal(t, 3, len(jobsTest.Items))
	err = sc.slurmCollectType(partitionsTest)
	assert.Equal(t, nil, err)
	assert.Equal(t, 2, len(partitionsTest.Items))
	err = sc.slurmCollectType(nodesTest)
	assert.Equal(t, nil, err)
	assert.Equal(t, 3, len(nodesTest.Items))

	interceptorFunc := interceptor.Funcs{
		List: func(ctx context.Context, list object.ObjectList, opts ...client.ListOption) error {
			return errors.New("error")
		},
	}
	sc.slurmClusters.Add(name, newFakeClientList(interceptorFunc))
	err = sc.slurmCollectType(jobsTest)
	assert.NotEqual(t, nil, err)
}

// TestCollect will test the Prometheus Collect method that implements
// the Collector interface.
func TestCollect(t *testing.T) {

	sc := newSlurmCollectorHelper()

	c := make(chan prometheus.Metric)
	var metric prometheus.Metric
	var numMetric int

	interceptorFunc := interceptor.Funcs{
		List: func(ctx context.Context, list object.ObjectList, opts ...client.ListOption) error {
			if list.GetType() == slurmtypes.ObjectTypeNode {
				return errors.New("error")
			}
			return nil
		},
	}
	sc.slurmClusters.Add(name, newFakeClientList(interceptorFunc))
	sc.Collect(c)

	interceptorFunc = interceptor.Funcs{
		List: func(ctx context.Context, list object.ObjectList, opts ...client.ListOption) error {
			if list.GetType() == slurmtypes.ObjectTypeJobInfo {
				return errors.New("error")
			}
			return nil
		},
	}
	sc.slurmClusters.Add(name, newFakeClientList(interceptorFunc))
	sc.Collect(c)

	interceptorFunc = interceptor.Funcs{
		List: func(ctx context.Context, list object.ObjectList, opts ...client.ListOption) error {
			if list.GetType() == slurmtypes.ObjectTypePartitionInfo {
				return errors.New("error")
			}
			return nil
		},
	}
	sc.slurmClusters.Add(name, newFakeClientList(interceptorFunc))
	sc.Collect(c)

	sc = newSlurmCollectorHelper()

	go func() {
		sc.Collect(c)
		close(c)
	}()
	for metric = range c {
		numMetric++
	}
	assert.NotNil(t, metric)
	assert.Equal(t, 41, numMetric)
}

// TestDescribe will test the Prometheus Describe method that implements
// the Collector interface. Verify the correct number of metric
// descriptions is returned from the channel.
func TestDescribe(t *testing.T) {
	c := make(chan *prometheus.Desc)
	var desc *prometheus.Desc
	var numDesc int
	sc := NewSlurmCollector(name, exporter_url, cacheFreq, false)

	go func() {
		sc.Describe(c)
		close(c)
	}()
	for desc = range c {
		numDesc++
		assert.NotNil(t, desc)
	}
	assert.NotNil(t, desc)
	assert.Equal(t, 26, numDesc)
}

// TestNewSlurmCollector will test that NewSlurmCollector
// returns the correct SlurmCollector.
func TestNewSlurmCollector(t *testing.T) {
	sc := NewSlurmCollector(name, exporter_url, cacheFreq, false)

	assert.Equal(t, exporter_url, sc.server)
	assert.Equal(t, name, sc.name)
	assert.Equal(t, cacheFreq, sc.cacheFreq)
	assert.Equal(t, false, sc.perUserMetrics)
}
