// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/pingcap/errcode"
	"github.com/pingcap/kvproto/pkg/metapb"
	"github.com/pingcap/kvproto/pkg/pdpb"
	log "github.com/sirupsen/logrus"
)

// StoreInfo contains information about a store.
type StoreInfo struct {
	meta  *metapb.Store
	stats *pdpb.StoreStats
	// Blocked means that the store is blocked from balance.
	blocked           bool
	leaderCount       int
	regionCount       int
	leaderSize        int64
	regionSize        int64
	pendingPeerCount  int
	lastHeartbeatTS   time.Time
	leaderWeight      float64
	regionWeight      float64
	rollingStoreStats *RollingStoreStats
}

// NewStoreInfo creates StoreInfo with meta data.
func NewStoreInfo(store *metapb.Store, opts ...StoreCreateOption) *StoreInfo {
	storeInfo := &StoreInfo{
		meta:              store,
		stats:             &pdpb.StoreStats{},
		leaderWeight:      1.0,
		regionWeight:      1.0,
		rollingStoreStats: newRollingStoreStats(),
	}
	for _, opt := range opts {
		opt(storeInfo)
	}
	return storeInfo
}

// Clone creates a copy of current StoreInfo.
func (s *StoreInfo) Clone(opts ...StoreCreateOption) *StoreInfo {
	store := &StoreInfo{
		meta:              s.meta,
		stats:             s.stats,
		blocked:           s.blocked,
		leaderCount:       s.leaderCount,
		regionCount:       s.regionCount,
		leaderSize:        s.leaderSize,
		regionSize:        s.regionSize,
		pendingPeerCount:  s.pendingPeerCount,
		lastHeartbeatTS:   s.lastHeartbeatTS,
		leaderWeight:      s.leaderWeight,
		regionWeight:      s.regionWeight,
		rollingStoreStats: s.rollingStoreStats,
	}

	for _, opt := range opts {
		opt(store)
	}
	return store
}

// IsBlocked returns if the store is blocked.
func (s *StoreInfo) IsBlocked() bool {
	return s.blocked
}

// IsUp checks if the store's state is Up.
func (s *StoreInfo) IsUp() bool {
	return s.GetState() == metapb.StoreState_Up
}

// IsOffline checks if the store's state is Offline.
func (s *StoreInfo) IsOffline() bool {
	return s.GetState() == metapb.StoreState_Offline
}

// IsTombstone checks if the store's state is Tombstone.
func (s *StoreInfo) IsTombstone() bool {
	return s.GetState() == metapb.StoreState_Tombstone
}

// DownTime returns the time elapsed since last heartbeat.
func (s *StoreInfo) DownTime() time.Duration {
	return time.Since(s.GetLastHeartbeatTS())
}

// GetMeta returns the meta information of the store.
func (s *StoreInfo) GetMeta() *metapb.Store {
	return s.meta
}

// GetState returns the state of the store.
func (s *StoreInfo) GetState() metapb.StoreState {
	return s.meta.GetState()
}

// GetAddress returns the address of the store.
func (s *StoreInfo) GetAddress() string {
	return s.meta.GetAddress()
}

// GetVersion returns the version of the store.
func (s *StoreInfo) GetVersion() string {
	return s.meta.GetVersion()
}

// GetLabels returns the labels of the store.
func (s *StoreInfo) GetLabels() []*metapb.StoreLabel {
	return s.meta.GetLabels()
}

// GetID returns the ID of the store.
func (s *StoreInfo) GetID() uint64 {
	return s.meta.GetId()
}

// GetStoreStats returns the statistics information of the store.
func (s *StoreInfo) GetStoreStats() *pdpb.StoreStats {
	return s.stats
}

// GetCapacity returns the capacity size of the store.
func (s *StoreInfo) GetCapacity() uint64 {
	return s.stats.GetCapacity()
}

// GetAvailable returns the available size of the store.
func (s *StoreInfo) GetAvailable() uint64 {
	return s.stats.GetAvailable()
}

// GetUsedSize returns the used size of the store.
func (s *StoreInfo) GetUsedSize() uint64 {
	return s.stats.GetUsedSize()
}

// GetBytesWritten returns the bytes written for the store during this period.
func (s *StoreInfo) GetBytesWritten() uint64 {
	return s.stats.GetBytesWritten()
}

// GetBytesRead returns the bytes read for the store during this period.
func (s *StoreInfo) GetBytesRead() uint64 {
	return s.stats.GetBytesRead()
}

// GetKeysWritten returns the keys written for the store during this period.
func (s *StoreInfo) GetKeysWritten() uint64 {
	return s.stats.GetKeysWritten()
}

// GetKeysRead returns the keys read for the store during this period.
func (s *StoreInfo) GetKeysRead() uint64 {
	return s.stats.GetKeysRead()
}

// GetIsBusy returns if the store is busy.
func (s *StoreInfo) GetIsBusy() bool {
	return s.stats.GetIsBusy()
}

// GetSendingSnapCount returns the current sending snapshot count of the store.
func (s *StoreInfo) GetSendingSnapCount() uint32 {
	return s.stats.GetSendingSnapCount()
}

// GetReceivingSnapCount returns the current receiving snapshot count of the store.
func (s *StoreInfo) GetReceivingSnapCount() uint32 {
	return s.stats.GetReceivingSnapCount()
}

// GetApplyingSnapCount returns the current applying snapshot count of the store.
func (s *StoreInfo) GetApplyingSnapCount() uint32 {
	return s.stats.GetApplyingSnapCount()
}

// GetStartTime returns the start time of the store.
func (s *StoreInfo) GetStartTime() uint32 {
	return s.stats.GetStartTime()
}

// GetLeaderCount returns the leader count of the store.
func (s *StoreInfo) GetLeaderCount() int {
	return s.leaderCount
}

// GetRegionCount returns the Region count of the store.
func (s *StoreInfo) GetRegionCount() int {
	return s.regionCount
}

// GetLeaderSize returns the leader size of the store.
func (s *StoreInfo) GetLeaderSize() int64 {
	return s.leaderSize
}

// GetRegionSize returns the Region size of the store.
func (s *StoreInfo) GetRegionSize() int64 {
	return s.regionSize
}

// GetPendingPeerCount returns the pending peer count of the store.
func (s *StoreInfo) GetPendingPeerCount() int {
	return s.pendingPeerCount
}

// GetLeaderWeight returns the leader weight of the store.
func (s *StoreInfo) GetLeaderWeight() float64 {
	return s.leaderWeight
}

// GetRegionWeight returns the Region weight of the store.
func (s *StoreInfo) GetRegionWeight() float64 {
	return s.regionWeight
}

// GetLastHeartbeatTS returns the last heartbeat timestamp of the store.
func (s *StoreInfo) GetLastHeartbeatTS() time.Time {
	return s.lastHeartbeatTS
}

// GetRollingStoreStats returns the rolling statistics of the store.
func (s *StoreInfo) GetRollingStoreStats() *RollingStoreStats {
	return s.rollingStoreStats
}

const minWeight = 1e-6
const maxScore = 1024 * 1024 * 1024

// LeaderScore returns the store's leader score: leaderSize / leaderWeight.
func (s *StoreInfo) LeaderScore(delta int64) float64 {
	return float64(s.GetLeaderSize()+delta) / math.Max(s.GetLeaderWeight(), minWeight)
}

// RegionScore returns the store's region score.
func (s *StoreInfo) RegionScore(highSpaceRatio, lowSpaceRatio float64, delta int64) float64 {
	var score float64
	var amplification float64
	available := float64(s.GetAvailable()) / (1 << 20)
	used := float64(s.GetUsedSize()) / (1 << 20)
	capacity := float64(s.GetCapacity()) / (1 << 20)

	if s.GetRegionSize() == 0 {
		amplification = 1
	} else {
		// because of rocksdb compression, region size is larger than actual used size
		amplification = float64(s.GetRegionSize()) / used
	}

	// highSpaceBound is the lower bound of the high space stage.
	highSpaceBound := (1 - highSpaceRatio) * capacity
	// lowSpaceBound is the upper bound of the low space stage.
	lowSpaceBound := (1 - lowSpaceRatio) * capacity
	if available-float64(delta)/amplification >= highSpaceBound {
		score = float64(s.GetRegionSize() + delta)
	} else if available-float64(delta)/amplification <= lowSpaceBound {
		score = maxScore - (available - float64(delta)/amplification)
	} else {
		// to make the score function continuous, we use linear function y = k * x + b as transition period
		// from above we know that there are two points must on the function image
		// note that it is possible that other irrelative files occupy a lot of storage, so capacity == available + used + irrelative
		// and we regarded irrelative as a fixed value.
		// Then amp = size / used = size / (capacity - irrelative - available)
		//
		// When available == highSpaceBound,
		// we can conclude that size = (capacity - irrelative - highSpaceBound) * amp = (used + available - highSpaceBound) * amp
		// Similarly, when available == lowSpaceBound,
		// we can conclude that size = (capacity - irrelative - lowSpaceBound) * amp = (used + available - lowSpaceBound) * amp
		// These are the two fixed points' x-coordinates, and y-coordinates which can be easily obtained from the above two functions.
		x1, y1 := (used+available-highSpaceBound)*amplification, (used+available-highSpaceBound)*amplification
		x2, y2 := (used+available-lowSpaceBound)*amplification, maxScore-lowSpaceBound

		k := (y2 - y1) / (x2 - x1)
		b := y1 - k*x1
		score = k*float64(s.GetRegionSize()+delta) + b
	}

	return score / math.Max(s.GetRegionWeight(), minWeight)
}

// StorageSize returns store's used storage size reported from tikv.
func (s *StoreInfo) StorageSize() uint64 {
	return s.GetUsedSize()
}

// AvailableRatio is store's freeSpace/capacity.
func (s *StoreInfo) AvailableRatio() float64 {
	if s.GetCapacity() == 0 {
		return 0
	}
	return float64(s.GetAvailable()) / float64(s.GetCapacity())
}

// IsLowSpace checks if the store is lack of space.
func (s *StoreInfo) IsLowSpace(lowSpaceRatio float64) bool {
	return s.GetStoreStats() != nil && s.AvailableRatio() < 1-lowSpaceRatio
}

// ResourceCount reutrns count of leader/region in the store.
func (s *StoreInfo) ResourceCount(kind ResourceKind) uint64 {
	switch kind {
	case LeaderKind:
		return uint64(s.GetLeaderCount())
	case RegionKind:
		return uint64(s.GetRegionCount())
	default:
		return 0
	}
}

// ResourceSize returns size of leader/region in the store
func (s *StoreInfo) ResourceSize(kind ResourceKind) int64 {
	switch kind {
	case LeaderKind:
		return s.GetLeaderSize()
	case RegionKind:
		return s.GetRegionSize()
	default:
		return 0
	}
}

// ResourceScore reutrns score of leader/region in the store.
func (s *StoreInfo) ResourceScore(kind ResourceKind, highSpaceRatio, lowSpaceRatio float64, delta int64) float64 {
	switch kind {
	case LeaderKind:
		return s.LeaderScore(delta)
	case RegionKind:
		return s.RegionScore(highSpaceRatio, lowSpaceRatio, delta)
	default:
		return 0
	}
}

// ResourceWeight returns weight of leader/region in the score
func (s *StoreInfo) ResourceWeight(kind ResourceKind) float64 {
	switch kind {
	case LeaderKind:
		leaderWeight := s.GetLeaderWeight()
		if leaderWeight <= 0 {
			return minWeight
		}
		return leaderWeight
	case RegionKind:
		regionWeight := s.GetRegionWeight()
		if regionWeight <= 0 {
			return minWeight
		}
		return regionWeight
	default:
		return 0
	}
}

// GetStartTS returns the start timestamp.
func (s *StoreInfo) GetStartTS() time.Time {
	return time.Unix(int64(s.GetStartTime()), 0)
}

// GetUptime returns the uptime.
func (s *StoreInfo) GetUptime() time.Duration {
	uptime := s.GetLastHeartbeatTS().Sub(s.GetStartTS())
	if uptime > 0 {
		return uptime
	}
	return 0
}

var (
	// If a store's last heartbeat is storeDisconnectDuration ago, the store will
	// be marked as disconnected state. The value should be greater than tikv's
	// store heartbeat interval (default 10s).
	storeDisconnectDuration = 20 * time.Second
	storeUnhealthDuration   = 10 * time.Minute
)

// IsDisconnected checks if a store is disconnected, which means PD misses
// tikv's store heartbeat for a short time, maybe caused by process restart or
// temporary network failure.
func (s *StoreInfo) IsDisconnected() bool {
	return s.DownTime() > storeDisconnectDuration
}

// IsUnhealth checks if a store is unhealth.
func (s *StoreInfo) IsUnhealth() bool {
	return s.DownTime() > storeUnhealthDuration
}

// GetLabelValue returns a label's value (if exists).
func (s *StoreInfo) GetLabelValue(key string) string {
	for _, label := range s.GetLabels() {
		if strings.EqualFold(label.GetKey(), key) {
			return label.GetValue()
		}
	}
	return ""
}

// CompareLocation compares 2 stores' labels and returns at which level their
// locations are different. It returns -1 if they are at the same location.
func (s *StoreInfo) CompareLocation(other *StoreInfo, labels []string) int {
	for i, key := range labels {
		v1, v2 := s.GetLabelValue(key), other.GetLabelValue(key)
		// If label is not set, the store is considered at the same location
		// with any other store.
		if v1 != "" && v2 != "" && !strings.EqualFold(v1, v2) {
			return i
		}
	}
	return -1
}

// MergeLabels merges the passed in labels with origins, overriding duplicated
// ones.
func (s *StoreInfo) MergeLabels(labels []*metapb.StoreLabel) []*metapb.StoreLabel {
	storeLabels := s.GetLabels()
L:
	for _, newLabel := range labels {
		for _, label := range storeLabels {
			if strings.EqualFold(label.Key, newLabel.Key) {
				label.Value = newLabel.Value
				continue L
			}
		}
		storeLabels = append(storeLabels, newLabel)
	}
	return storeLabels
}

// StoreHotRegionInfos : used to get human readable description for hot regions.
type StoreHotRegionInfos struct {
	AsPeer   StoreHotRegionsStat `json:"as_peer"`
	AsLeader StoreHotRegionsStat `json:"as_leader"`
}

// StoreHotRegionsStat used to record the hot region statistics group by store
type StoreHotRegionsStat map[uint64]*HotRegionsStat

type storeNotFoundErr struct {
	storeID uint64
}

func (e storeNotFoundErr) Error() string {
	return fmt.Sprintf("store %v not found", e.storeID)
}

// NewStoreNotFoundErr is for log of store not found
func NewStoreNotFoundErr(storeID uint64) errcode.ErrorCode {
	return errcode.NewNotFoundErr(storeNotFoundErr{storeID})
}

// StoresInfo contains information about all stores.
type StoresInfo struct {
	stores         map[uint64]*StoreInfo
	bytesReadRate  float64
	bytesWriteRate float64
}

// NewStoresInfo create a StoresInfo with map of storeID to StoreInfo
func NewStoresInfo() *StoresInfo {
	return &StoresInfo{
		stores: make(map[uint64]*StoreInfo),
	}
}

// GetStore returns a copy of the StoreInfo with the specified storeID.
func (s *StoresInfo) GetStore(storeID uint64) *StoreInfo {
	store, ok := s.stores[storeID]
	if !ok {
		return nil
	}
	return store
}

// TakeStore returns the point of the origin StoreInfo with the specified storeID.
func (s *StoresInfo) TakeStore(storeID uint64) *StoreInfo {
	store, ok := s.stores[storeID]
	if !ok {
		return nil
	}
	return store
}

// SetStore sets a StoreInfo with storeID.
func (s *StoresInfo) SetStore(store *StoreInfo) {
	s.stores[store.GetID()] = store
	store.GetRollingStoreStats().Observe(store.GetStoreStats())
	s.updateTotalBytesReadRate()
	s.updateTotalBytesWriteRate()
}

// BlockStore blocks a StoreInfo with storeID.
func (s *StoresInfo) BlockStore(storeID uint64) errcode.ErrorCode {
	op := errcode.Op("store.block")
	store, ok := s.stores[storeID]
	if !ok {
		return op.AddTo(NewStoreNotFoundErr(storeID))
	}
	if store.IsBlocked() {
		return op.AddTo(StoreBlockedErr{StoreID: storeID})
	}
	s.stores[storeID] = store.Clone(SetStoreBlock())
	return nil
}

// UnblockStore unblocks a StoreInfo with storeID.
func (s *StoresInfo) UnblockStore(storeID uint64) {
	store, ok := s.stores[storeID]
	if !ok {
		log.Fatalf("store %d is unblocked, but it is not found", storeID)
	}
	s.stores[storeID] = store.Clone(SetStoreUnBlock())
}

// GetStores gets a complete set of StoreInfo.
func (s *StoresInfo) GetStores() []*StoreInfo {
	stores := make([]*StoreInfo, 0, len(s.stores))
	for _, store := range s.stores {
		stores = append(stores, store)
	}
	return stores
}

// GetMetaStores gets a complete set of metapb.Store.
func (s *StoresInfo) GetMetaStores() []*metapb.Store {
	stores := make([]*metapb.Store, 0, len(s.stores))
	for _, store := range s.stores {
		stores = append(stores, store.GetMeta())
	}
	return stores
}

// GetStoreCount returns the total count of storeInfo.
func (s *StoresInfo) GetStoreCount() int {
	return len(s.stores)
}

// SetLeaderCount sets the leader count to a storeInfo.
func (s *StoresInfo) SetLeaderCount(storeID uint64, leaderCount int) {
	if store, ok := s.stores[storeID]; ok {
		s.stores[storeID] = store.Clone(SetLeaderCount(leaderCount))
	}
}

// SetRegionCount sets the region count to a storeInfo.
func (s *StoresInfo) SetRegionCount(storeID uint64, regionCount int) {
	if store, ok := s.stores[storeID]; ok {
		s.stores[storeID] = store.Clone(SetRegionCount(regionCount))
	}
}

// SetPendingPeerCount sets the pending count to a storeInfo.
func (s *StoresInfo) SetPendingPeerCount(storeID uint64, pendingPeerCount int) {
	if store, ok := s.stores[storeID]; ok {
		s.stores[storeID] = store.Clone(SetPendingPeerCount(pendingPeerCount))
	}
}

// SetLeaderSize sets the leader size to a storeInfo.
func (s *StoresInfo) SetLeaderSize(storeID uint64, leaderSize int64) {
	if store, ok := s.stores[storeID]; ok {
		s.stores[storeID] = store.Clone(SetLeaderSize(leaderSize))
	}
}

// SetRegionSize sets the region size to a storeInfo.
func (s *StoresInfo) SetRegionSize(storeID uint64, regionSize int64) {
	if store, ok := s.stores[storeID]; ok {
		s.stores[storeID] = store.Clone(SetRegionSize(regionSize))
	}
}

// UpdateStoreStatusLocked updates the information of the store.
func (s *StoresInfo) UpdateStoreStatusLocked(storeID uint64, leaderCount int, regionCount int, pendingPeerCount int, leaderSize int64, regionSize int64) {
	if store, ok := s.stores[storeID]; ok {
		newStore := store.Clone(SetLeaderCount(leaderCount),
			SetRegionCount(regionCount),
			SetPendingPeerCount(pendingPeerCount),
			SetLeaderSize(leaderSize),
			SetRegionSize(regionSize))
		s.SetStore(newStore)
	}
}

func (s *StoresInfo) updateTotalBytesWriteRate() {
	var totalBytesWirteRate float64
	for _, s := range s.stores {
		if s.IsUp() {
			totalBytesWirteRate += s.GetRollingStoreStats().GetBytesWriteRate()
		}
	}
	s.bytesWriteRate = totalBytesWirteRate
}

// TotalBytesWriteRate returns the total written bytes rate of all StoreInfo.
func (s *StoresInfo) TotalBytesWriteRate() float64 {
	return s.bytesWriteRate
}

func (s *StoresInfo) updateTotalBytesReadRate() {
	var totalBytesReadRate float64
	for _, s := range s.stores {
		if s.IsUp() {
			totalBytesReadRate += s.GetRollingStoreStats().GetBytesReadRate()
		}
	}
	s.bytesReadRate = totalBytesReadRate
}

// TotalBytesReadRate returns the total read bytes rate of all StoreInfo.
func (s *StoresInfo) TotalBytesReadRate() float64 {
	return s.bytesReadRate
}

// GetStoresBytesWriteStat returns the bytes write stat of all StoreInfo.
func (s *StoresInfo) GetStoresBytesWriteStat() map[uint64]uint64 {
	res := make(map[uint64]uint64, len(s.stores))
	for _, s := range s.stores {
		res[s.GetID()] = uint64(s.GetRollingStoreStats().GetBytesWriteRate())
	}
	return res
}

// GetStoresBytesReadStat returns the bytes read stat of all StoreInfo.
func (s *StoresInfo) GetStoresBytesReadStat() map[uint64]uint64 {
	res := make(map[uint64]uint64, len(s.stores))
	for _, s := range s.stores {
		res[s.GetID()] = uint64(s.GetRollingStoreStats().GetBytesReadRate())
	}
	return res
}

// GetStoresKeysWriteStat returns the keys write stat of all StoreInfo.
func (s *StoresInfo) GetStoresKeysWriteStat() map[uint64]uint64 {
	res := make(map[uint64]uint64, len(s.stores))
	for _, s := range s.stores {
		res[s.GetID()] = uint64(s.GetRollingStoreStats().GetKeysWriteRate())
	}
	return res
}

// GetStoresKeysReadStat returns the bytes read stat of all StoreInfo.
func (s *StoresInfo) GetStoresKeysReadStat() map[uint64]uint64 {
	res := make(map[uint64]uint64, len(s.stores))
	for _, s := range s.stores {
		res[s.GetID()] = uint64(s.GetRollingStoreStats().GetKeysReadRate())
	}
	return res
}

// RollingStoreStats are multiple sets of recent historical records with specified windows size.
type RollingStoreStats struct {
	sync.RWMutex
	bytesWriteRate *RollingStats
	bytesReadRate  *RollingStats
	keysWriteRate  *RollingStats
	keysReadRate   *RollingStats
}

const storeStatsRollingWindows = 3

func newRollingStoreStats() *RollingStoreStats {
	return &RollingStoreStats{
		bytesWriteRate: NewRollingStats(storeStatsRollingWindows),
		bytesReadRate:  NewRollingStats(storeStatsRollingWindows),
		keysWriteRate:  NewRollingStats(storeStatsRollingWindows),
		keysReadRate:   NewRollingStats(storeStatsRollingWindows),
	}
}

// Observe records current statistics.
func (r *RollingStoreStats) Observe(stats *pdpb.StoreStats) {
	interval := stats.GetInterval().GetEndTimestamp() - stats.GetInterval().GetStartTimestamp()
	if interval == 0 {
		return
	}
	r.Lock()
	defer r.Unlock()
	r.bytesWriteRate.Add(float64(stats.BytesWritten / interval))
	r.bytesReadRate.Add(float64(stats.BytesRead / interval))
	r.keysWriteRate.Add(float64(stats.KeysWritten / interval))
	r.keysReadRate.Add(float64(stats.KeysRead / interval))
}

// GetBytesWriteRate returns the bytes write rate.
func (r *RollingStoreStats) GetBytesWriteRate() float64 {
	r.RLock()
	defer r.RUnlock()
	return r.bytesWriteRate.Median()
}

// GetBytesReadRate returns the bytes read rate.
func (r *RollingStoreStats) GetBytesReadRate() float64 {
	r.RLock()
	defer r.RUnlock()
	return r.bytesReadRate.Median()
}

// GetKeysWriteRate returns the keys write rate.
func (r *RollingStoreStats) GetKeysWriteRate() float64 {
	r.RLock()
	defer r.RUnlock()
	return r.keysWriteRate.Median()
}

// GetKeysReadRate returns the keys read rate.
func (r *RollingStoreStats) GetKeysReadRate() float64 {
	r.RLock()
	defer r.RUnlock()
	return r.keysReadRate.Median()
}
