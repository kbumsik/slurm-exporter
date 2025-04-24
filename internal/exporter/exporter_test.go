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

	"github.com/SlinkyProject/slurm-client/pkg/client"
	"github.com/SlinkyProject/slurm-client/pkg/client/fake"
	"github.com/SlinkyProject/slurm-client/pkg/client/interceptor"
	"github.com/SlinkyProject/slurm-client/pkg/object"

	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"
)

var (
	exporter_url = "http://localhost:8080"
	cacheFreq    = 5 * time.Second
)

// newSlurmCollectorHelper will initialize a Slurm Collector for
// metrics and will also load a predefined set of objects into
// the cache of a fake Slurm client.
func newSlurmCollectorHelper() SlurmCollector {
	jobs := &slurmtypes.V0041JobInfoList{}
	partitions := &slurmtypes.V0041PartitionInfoList{}
	nodes := &slurmtypes.V0041NodeList{}

	jobs.AppendItem(jobInfoA)
	jobs.AppendItem(jobInfoB)
	jobs.AppendItem(jobInfoC)
	partitions.AppendItem(partitionA)
	partitions.AppendItem(partitionB)
	nodes.AppendItem(nodeA)
	nodes.AppendItem(nodeB)
	nodes.AppendItem(nodeC)

	slurmClient := newFakeClientList(interceptor.Funcs{}, jobs, partitions, nodes)
	sc := NewSlurmCollector(slurmClient, true)
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
	os.Setenv("SLURM_JWT", "foo")
	c, err := NewSlurmClient(exporter_url, cacheFreq)
	assert.Nil(t, err)
	sc := NewSlurmCollector(c, false)

	jobs := &slurmtypes.V0041JobInfoList{}
	partitions := &slurmtypes.V0041PartitionInfoList{}
	nodes := &slurmtypes.V0041NodeList{}

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
	var c client.Client
	var err error

	os.Unsetenv("SLURM_JWT")
	c, err = NewSlurmClient(exporter_url, cacheFreq)
	assert.NotNil(t, err)
	assert.Nil(t, c)

	os.Setenv("SLURM_JWT", "")
	c, err = NewSlurmClient(exporter_url, cacheFreq)
	assert.NotNil(t, err)
	assert.Nil(t, c)

	os.Setenv("SLURM_JWT", "foo")
	c, err = NewSlurmClient(exporter_url, cacheFreq)
	assert.Nil(t, err)
	assert.NotNil(t, c)
}

// TestSlurmCollectType will test that slurmCollectType returns
// the correct data from a Slurm client. A fake Slurm
// client is used and preloaded with a cache of objects. In order
// to simulate error conditions, interceptor functions are used.
func TestSlurmCollectType(t *testing.T) {

	sc := newSlurmCollectorHelper()

	jobsTest := &slurmtypes.V0041JobInfoList{}
	partitionsTest := &slurmtypes.V0041PartitionInfoList{}
	nodesTest := &slurmtypes.V0041NodeList{}

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
	sc.slurmClient = newFakeClientList(interceptorFunc)
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
			if list.GetType() == slurmtypes.ObjectTypeV0041Node {
				return errors.New("error")
			}
			return nil
		},
	}
	sc.slurmClient = newFakeClientList(interceptorFunc)
	sc.Collect(c)

	interceptorFunc = interceptor.Funcs{
		List: func(ctx context.Context, list object.ObjectList, opts ...client.ListOption) error {
			if list.GetType() == slurmtypes.ObjectTypeV0041JobInfo {
				return errors.New("error")
			}
			return nil
		},
	}
	sc.slurmClient = newFakeClientList(interceptorFunc)
	sc.Collect(c)

	interceptorFunc = interceptor.Funcs{
		List: func(ctx context.Context, list object.ObjectList, opts ...client.ListOption) error {
			if list.GetType() == slurmtypes.ObjectTypeV0041PartitionInfo {
				return errors.New("error")
			}
			return nil
		},
	}
	sc.slurmClient = newFakeClientList(interceptorFunc)
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
	cl, err := NewSlurmClient(exporter_url, cacheFreq)
	assert.Nil(t, err)
	sc := NewSlurmCollector(cl, false)

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

// TestParseNodeRange will test the parseNodeRange function with various test cases.
func TestParseNodeRange(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
		wantErr  bool
	}{
		{
			name:     "single node",
			input:    "node1",
			expected: []string{"node1"},
			wantErr:  false,
		},
		{
			name:     "comma separated nodes",
			input:    "node1,node2,node3",
			expected: []string{"node1", "node2", "node3"},
			wantErr:  false,
		},
		{
			name:     "simple range",
			input:    "compute-[1-3]",
			expected: []string{"compute-1", "compute-2", "compute-3"},
			wantErr:  false,
		},
		{
			name:     "range with padding",
			input:    "compute-[01-03]",
			expected: []string{"compute-01", "compute-02", "compute-03"},
			wantErr:  false,
		},
		{
			name:     "list",
			input:    "compute-[1,3,5]",
			expected: []string{"compute-1", "compute-3", "compute-5"},
			wantErr:  false,
		},
		{
			name:     "list with padding",
			input:    "compute-[01,03,05]",
			expected: []string{"compute-01", "compute-03", "compute-05"},
			wantErr:  false,
		},
		{
			name:     "mixed range and list",
			input:    "compute-[1,3-5,7]",
			expected: []string{"compute-1", "compute-3", "compute-4", "compute-5", "compute-7"},
			wantErr:  false,
		},
		{
			name:     "mixed range and list with padding",
			input:    "compute-[01,03-05,07]",
			expected: []string{"compute-01", "compute-03", "compute-04", "compute-05", "compute-07"},
			wantErr:  false,
		},
		{
			name:     "multiple ranges",
			input:    "compute-[1-2],login-[01-02]",
			expected: []string{"compute-1", "compute-2", "login-01", "login-02"},
			wantErr:  false,
		},
		{
			name:     "multiple ranges complex",
			input:    "prefix1-[1,3-4],node5,prefix2-[01-02]",
			expected: []string{"prefix1-1", "prefix1-3", "prefix1-4", "node5", "prefix2-01", "prefix2-02"},
			wantErr:  false,
		},
		{
			name:     "prefix before comma before range",
			input:    "node0,compute-[1-2]",
			expected: []string{"node0", "compute-1", "compute-2"},
			wantErr:  false,
		},
		{
			name:     "suffix_after_range",
			input:    "compute-[1-2]-gpu", // Note: range must be the last element
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "multiple brackets with suffix",
			input:    "compute-[1-2],login-[1-2]-node", // suffix only applies after last group
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "complex mix",
			input:    "gpu-[01-02],cpu-[1,3-4],node10,mgmt-[1-1]",
			expected: []string{"gpu-01", "gpu-02", "cpu-1", "cpu-3", "cpu-4", "node10", "mgmt-1"},
			wantErr:  false,
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "whitespace input",
			input:    "  ",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "just commas",
			input:    ",,",
			expected: nil,
			wantErr:  true,
		},
		{
			name:    "invalid range start > end",
			input:   "compute-[5-1]",
			wantErr: true,
		},
		{
			name:    "invalid range non-numeric start",
			input:   "compute-[a-5]",
			wantErr: true,
		},
		{
			name:    "invalid range non-numeric end",
			input:   "compute-[1-b]",
			wantErr: true,
		},
		{
			name:    "invalid list item",
			input:   "compute-[1,abc,3]",
			wantErr: true,
		},
		{
			name:    "invalid range format",
			input:   "compute-[1-]",
			wantErr: true,
		},
		{
			name:     "empty brackets",
			input:    "compute-[]",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "brackets inside node name before range",
			input:    "node[1]-[1-2]",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "brackets inside final node name segment",
			input:    "compute-[1-2],node[3]",
			expected: []string{"compute-1", "compute-2", "node3"},
			wantErr:  false,
		},
		{
			name:     "special case (null)",
			input:    "(null)",
			expected: []string{"(null)"},
			wantErr:  false,
		},
		{
			name:     "special case All",
			input:    "All",
			expected: []string{"All"},
			wantErr:  false,
		},
		{
			name:     "nested brackets",
			input:    "compute-[1-[2]]",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "malformed bracket expression",
			input:    "compute-[1-2",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "Nodes with varying padding lengths",
			input:    "node-[1,10,100]",
			expected: []string{"node-001", "node-010", "node-100"},
			wantErr:  false,
		},
		{
			name:     "Range crossing padding boundary",
			input:    "node-[8-12]",
			expected: []string{"node-08", "node-09", "node-10", "node-11", "node-12"},
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseNodeRange(tc.input)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Use ElementsMatch because order isn't guaranteed by the spec, though current impl is ordered.
				assert.ElementsMatch(t, tc.expected, got)
			}
		})
	}
}

// TestNewSlurmCollector will test that NewSlurmCollector
// returns the correct SlurmCollector.
func TestNewSlurmCollector(t *testing.T) {
	c, err := NewSlurmClient(exporter_url, cacheFreq)
	assert.Nil(t, err)
	assert.NotNil(t, c)
	sc := NewSlurmCollector(c, false)
	assert.NotNil(t, sc)
	assert.Equal(t, false, sc.perUserMetrics)
}
