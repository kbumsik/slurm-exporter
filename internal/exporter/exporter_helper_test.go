// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"

	api "github.com/SlinkyProject/slurm-client/api/v0041"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"
)

var (
	jobStateSetPending    = make(set.Set[api.V0041JobInfoJobState]).Insert(api.V0041JobInfoJobStatePENDING)
	jobStateSetRunning    = make(set.Set[api.V0041JobInfoJobState]).Insert(api.V0041JobInfoJobStateRUNNING)
	partitionStateSetUp   = make(set.Set[api.V0041PartitionInfoPartitionState]).Insert(api.V0041PartitionInfoPartitionStateUP)
	partitionStateSetDown = make(set.Set[api.V0041PartitionInfoPartitionState]).Insert(api.V0041PartitionInfoPartitionStateDOWN)
	nodeStateSetIdle      = make(set.Set[api.V0041NodeState]).Insert(api.V0041NodeStateIDLE)
	nodeStateSetAllocated = make(set.Set[api.V0041NodeState]).Insert(api.V0041NodeStateALLOCATED)
	// nodeStateSetMisc is not a valid Slurm state for a node. It is used during testing to verify a NodeStates struct.
	nodeStateSetMisc = make(set.Set[api.V0041NodeState]).Insert(
		api.V0041NodeStateCOMPLETING,
		api.V0041NodeStateDOWN,
		api.V0041NodeStateDRAIN,
		api.V0041NodeStateERROR,
		api.V0041NodeStateMAINTENANCE,
		api.V0041NodeStateMIXED,
		api.V0041NodeStateRESERVED,
	)
	nodePartitionSetPurple      = make(set.Set[string]).Insert("purple")
	nodePartitionSetGreen       = make(set.Set[string]).Insert("green")
	nodePartitionSetPurpleGreen = make(set.Set[string]).Insert("purple").Insert("green")

	jobInfoA = &slurmtypes.V0041JobInfo{
		V0041JobInfo: api.V0041JobInfo{
			JobId:     ptr.To[int32](1),
			UserName:  ptr.To("slurm"),
			Partition: ptr.To("purple"),
			JobState:  ptr.To(jobStateSetPending.UnsortedList()),
			Cpus: &api.V0041Uint32NoValStruct{
				Number: ptr.To[int32](12),
				Set:    ptr.To(true),
			},
			NodeCount: &api.V0041Uint32NoValStruct{
				Number: ptr.To[int32](1),
				Set:    ptr.To(true),
			},
			Hold: ptr.To(false),
			EligibleTime: &api.V0041Uint64NoValStruct{
				Number: ptr.To[int64](1722609649),
				Set:    ptr.To(true),
			},
		},
	}
	jobInfoB = &slurmtypes.V0041JobInfo{
		V0041JobInfo: api.V0041JobInfo{
			JobId:     ptr.To[int32](2),
			UserName:  ptr.To("slurm"),
			Partition: ptr.To("green"),
			JobState:  ptr.To(jobStateSetRunning.UnsortedList()),
			Cpus: &api.V0041Uint32NoValStruct{
				Number: ptr.To[int32](12),
				Set:    ptr.To(true),
			},
			NodeCount: &api.V0041Uint32NoValStruct{
				Number: ptr.To[int32](1),
				Set:    ptr.To(true),
			},
			Hold: ptr.To(true),
			EligibleTime: &api.V0041Uint64NoValStruct{
				Number: ptr.To[int64](1722609649),
				Set:    ptr.To(true),
			},
		},
	}
	jobInfoC = &slurmtypes.V0041JobInfo{
		V0041JobInfo: api.V0041JobInfo{
			JobId:     ptr.To[int32](3),
			UserName:  ptr.To("slurm"),
			Partition: ptr.To("foo"),
			JobState:  ptr.To(jobStateSetPending.UnsortedList()),
			Cpus: &api.V0041Uint32NoValStruct{
				Number: ptr.To[int32](12),
				Set:    ptr.To(true),
			},
			NodeCount: &api.V0041Uint32NoValStruct{
				Number: ptr.To[int32](2),
				Set:    ptr.To(true),
			},
			Hold: ptr.To(false),
		},
	}
	jobInfoD = &slurmtypes.V0041JobInfo{
		V0041JobInfo: api.V0041JobInfo{
			JobId:     ptr.To[int32](4),
			UserName:  ptr.To("hal"),
			Partition: ptr.To("green"),
			JobState:  ptr.To(jobStateSetPending.UnsortedList()),
			Cpus: &api.V0041Uint32NoValStruct{
				Number: ptr.To[int32](12),
				Set:    ptr.To(true),
			},
			NodeCount: &api.V0041Uint32NoValStruct{
				Number: ptr.To[int32](2),
				Set:    ptr.To(true),
			},
			Hold: ptr.To(false),
		},
	}
	partitionA = &slurmtypes.V0041PartitionInfo{
		V0041PartitionInfo: api.V0041PartitionInfo{
			Name:     ptr.To("purple"),
			NodeSets: ptr.To("purple"),
			Nodes: &struct {
				AllowedAllocation *string "json:\"allowed_allocation,omitempty\""
				Configured        *string "json:\"configured,omitempty\""
				Total             *int32  "json:\"total,omitempty\""
			}{
				Total: ptr.To[int32](2),
			},
			Cpus: &struct {
				TaskBinding *int32 "json:\"task_binding,omitempty\""
				Total       *int32 "json:\"total,omitempty\""
			}{
				Total: ptr.To[int32](20),
			},
			Partition: &struct {
				State *[]api.V0041PartitionInfoPartitionState "json:\"state,omitempty\""
			}{
				State: ptr.To(partitionStateSetUp.UnsortedList()),
			},
		},
	}
	partitionB = &slurmtypes.V0041PartitionInfo{
		V0041PartitionInfo: api.V0041PartitionInfo{
			Name:     ptr.To("green"),
			NodeSets: ptr.To("green"),
			Nodes: &struct {
				AllowedAllocation *string "json:\"allowed_allocation,omitempty\""
				Configured        *string "json:\"configured,omitempty\""
				Total             *int32  "json:\"total,omitempty\""
			}{
				Total: ptr.To[int32](2),
			},
			Cpus: &struct {
				TaskBinding *int32 "json:\"task_binding,omitempty\""
				Total       *int32 "json:\"total,omitempty\""
			}{
				Total: ptr.To[int32](32),
			},
			Partition: &struct {
				State *[]api.V0041PartitionInfoPartitionState "json:\"state,omitempty\""
			}{
				State: ptr.To(partitionStateSetDown.UnsortedList()),
			},
		},
	}
	nodeA = &slurmtypes.V0041Node{
		V0041Node: api.V0041Node{
			Name:          ptr.To("kind-worker"),
			Address:       ptr.To("10.244.1.1"),
			Partitions:    ptr.To(nodePartitionSetPurple.UnsortedList()),
			State:         ptr.To(nodeStateSetIdle.UnsortedList()),
			Cpus:          ptr.To[int32](12),
			AllocCpus:     ptr.To[int32](0),
			AllocIdleCpus: ptr.To[int32](12),
		},
	}
	nodeB = &slurmtypes.V0041Node{
		V0041Node: api.V0041Node{
			Name:          ptr.To("kind-worker2"),
			Address:       ptr.To("10.244.1.2"),
			Partitions:    ptr.To(nodePartitionSetGreen.UnsortedList()),
			State:         ptr.To(nodeStateSetAllocated.UnsortedList()),
			Cpus:          ptr.To[int32](24),
			AllocCpus:     ptr.To[int32](12),
			AllocIdleCpus: ptr.To[int32](12),
		},
	}
	nodeC = &slurmtypes.V0041Node{
		V0041Node: api.V0041Node{
			Name:          ptr.To("kind-worker3"),
			Address:       ptr.To("10.244.1.3"),
			Partitions:    ptr.To(nodePartitionSetPurpleGreen.UnsortedList()),
			State:         ptr.To(nodeStateSetMisc.UnsortedList()),
			Cpus:          ptr.To[int32](8),
			AllocCpus:     ptr.To[int32](4),
			AllocIdleCpus: ptr.To[int32](4),
		},
	}
)
