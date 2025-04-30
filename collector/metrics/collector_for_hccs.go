/* Copyright(C) 2025-2025. Huawei Technologies Co.,Ltd. All rights reserved.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package metrics for general collector
package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"ascend-common/devmanager/common"
	colcommon "huawei.com/npu-exporter/v6/collector/common"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/utils/logger"
)

var (
	hccsTxDescs         []*prometheus.Desc
	hccsRxDescs         []*prometheus.Desc
	hccsErrDescs        []*prometheus.Desc
	hccsBWTxDescs       []*prometheus.Desc
	hccsBWRxDescs       []*prometheus.Desc
	hccsBWProfilingTime *prometheus.Desc = nil
	hccsBWTotalTx       *prometheus.Desc = nil
	hccsBWTotalRx       *prometheus.Desc = nil

	supportedHccsDevices = map[string]bool{
		common.Ascend910B:  true,
		common.Ascend910A3: true,
	}
)

const (
	// MaxHccsNum max hccs num
	MaxHccsNum int = 8
	// hccs info begin index, 1 or 2
	num1     = 1
	num2     = 2
	prefix   = "npu_chip_info_hccs_statistic_info_"
	bwPrefix = "npu_chip_info_hccs_bandwidth_info_"
)

// init add descs in init method
func init() {
	for i := 0; i < MaxHccsNum; i++ {
		index := strconv.Itoa(i)
		colcommon.BuildDescSlice(&hccsTxDescs, prefix+"tx_cnt_"+index, "transmitted message count for hccs "+index)
		colcommon.BuildDescSlice(&hccsRxDescs, prefix+"rx_cnt_"+index, "received message count for hccs "+index)
		colcommon.BuildDescSlice(&hccsErrDescs, prefix+"crc_err_cnt_"+index, "crc error count for hccs "+index)
		colcommon.BuildDescSlice(&hccsBWTxDescs, bwPrefix+"tx_"+index,
			"single-link transmission data bandwidth for hccs "+index)
		colcommon.BuildDescSlice(&hccsBWRxDescs, bwPrefix+"rx_"+index,
			"single-link receive data bandwidth for hccs "+index)
	}
	hccsBWProfilingTime = colcommon.BuildDesc(bwPrefix+"profiling_time", "sampling interval for hccs bandwidth")
	hccsBWTotalTx = colcommon.BuildDesc(bwPrefix+"total_tx", "total sent data bandwidth")
	hccsBWTotalRx = colcommon.BuildDesc(bwPrefix+"total_rx", "total received data bandwidth")
}

type hccsCache struct {
	chip      colcommon.HuaWeiAIChip
	timestamp time.Time
	// hccsStat hccs info of npu chip
	hccsStat *common.HccsStatisticInfo

	// hccsBW hccs bandwidth info of npu chip
	hccsBW *common.HccsBandwidthInfo
}

// HccsCollector collect hccs info
type HccsCollector struct {
	colcommon.MetricsCollectorAdapter
	hccsBeginIndex int
}

// IsSupported judge whether the collector is supported
func (c *HccsCollector) IsSupported(n *colcommon.NpuCollector) bool {
	isSupport := supportedHccsDevices[n.Dmgr.GetDevType()]
	logForUnSupportDevice(isSupport, n.Dmgr.GetDevType(), colcommon.GetCacheKey(c),
		"only 910B or 910A3 supports hccs info")
	return isSupport
}

// Describe description of the metric
func (c *HccsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range hccsTxDescs {
		ch <- desc
	}
	for _, desc := range hccsRxDescs {
		ch <- desc
	}
	for _, desc := range hccsErrDescs {
		ch <- desc
	}
	for _, desc := range hccsBWTxDescs {
		ch <- desc
	}
	for _, desc := range hccsBWRxDescs {
		ch <- desc
	}
	ch <- hccsBWProfilingTime
	ch <- hccsBWTotalTx
	ch <- hccsBWTotalRx
}

// CollectToCache collect the metric to cache
func (c *HccsCollector) CollectToCache(n *colcommon.NpuCollector, chipList []colcommon.HuaWeiAIChip) {
	for _, chip := range chipList {
		logicID := chip.LogicID
		hccsStatisticInfo, err := n.Dmgr.GetHccsStatisticInfo(logicID)
		handleErr(err, colcommon.DomainForHccs, logicID)

		hccsBandwidthInfo, err := n.Dmgr.GetHccsBandwidthInfo(logicID)
		handleErr(err, colcommon.DomainForHccsBW, logicID)
		c.LocalCache.Store(chip.PhyId, hccsCache{
			chip:      chip,
			timestamp: time.Now(),
			hccsStat:  hccsStatisticInfo,
			hccsBW:    hccsBandwidthInfo},
		)
	}

	colcommon.UpdateCache[hccsCache](n, colcommon.GetCacheKey(c), &c.LocalCache)
}

// PreCollect pre collect hccs info
func (c *HccsCollector) PreCollect(n *colcommon.NpuCollector, chipList []colcommon.HuaWeiAIChip) {
	if len(chipList) == 0 {
		return
	}
	chipOne := chipList[0]
	devType := n.Dmgr.GetDevType()
	if devType == common.Ascend910B || common.IsA900A3SuperPod(chipOne.MainBoardId) ||
		common.Is800IA3Chip(chipOne.MainBoardId) {
		// A2 or A900A3 SuperPod or 800IA3 begin at 1st bit
		c.hccsBeginIndex = num1
	} else if common.IsA9000A3SuperPod(chipOne.MainBoardId) {
		// A9000A3SuperPod begin at 2nd bit
		c.hccsBeginIndex = num2
	} else {
		logger.LogfWithOptions(logger.ErrorLevel, logger.LogOptions{Domain: "hccs", ID: "0"},
			"not support main board id:%d", chipOne.MainBoardId)
	}
}

// UpdatePrometheus update prometheus
func (c *HccsCollector) UpdatePrometheus(ch chan<- prometheus.Metric, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) {

	updateSingleChip := func(chipWithVnpu colcommon.HuaWeiAIChip, cache hccsCache, cardLabel []string) {
		timestamp := cache.timestamp
		promUpdateHccsStatisticInfo(ch, cache, c, timestamp, cardLabel)
		promUpdateHccsBwInfo(ch, cache, c, timestamp, cardLabel)
	}
	updateFrame[hccsCache](colcommon.GetCacheKey(c), n, containerMap, chips, updateSingleChip)
}

func promUpdateHccsBwInfo(ch chan<- prometheus.Metric, cache hccsCache, c *HccsCollector,
	timestamp time.Time, cardLabel []string) {
	bandwidthInfo := cache.hccsBW
	if bandwidthInfo == nil {
		return
	}
	if c.hccsBeginIndex < 0 {
		logger.Errorf("invalid hccsBeginIndex %v", c.hccsBeginIndex)
		return
	}
	for i := c.hccsBeginIndex; i < MaxHccsNum; i++ {
		doUpdateMetric(ch, timestamp, bandwidthInfo.TxBandwidth[i], cardLabel, hccsBWTxDescs[i])
		doUpdateMetric(ch, timestamp, bandwidthInfo.RxBandwidth[i], cardLabel, hccsBWRxDescs[i])
	}
	doUpdateMetric(ch, timestamp, bandwidthInfo.ProfilingTime, cardLabel, hccsBWProfilingTime)
	doUpdateMetric(ch, timestamp, bandwidthInfo.TotalTxbw, cardLabel, hccsBWTotalTx)
	doUpdateMetric(ch, timestamp, bandwidthInfo.TotalRxbw, cardLabel, hccsBWTotalRx)
}

func promUpdateHccsStatisticInfo(ch chan<- prometheus.Metric, cache hccsCache, c *HccsCollector,
	timestamp time.Time, cardLabel []string) {
	statisticInfo := cache.hccsStat

	if statisticInfo == nil {
		return
	}
	if c.hccsBeginIndex < 0 {
		logger.Errorf("invalid hccsBeginIndex %v", c.hccsBeginIndex)
		return
	}
	for i := c.hccsBeginIndex; i < MaxHccsNum; i++ {
		doUpdateMetric(ch, timestamp, statisticInfo.TxCnt[i], cardLabel, hccsTxDescs[i])
		doUpdateMetric(ch, timestamp, statisticInfo.RxCnt[i], cardLabel, hccsRxDescs[i])
		doUpdateMetric(ch, timestamp, statisticInfo.CrcErrCnt[i], cardLabel, hccsErrDescs[i])
	}
}

// UpdateTelegraf update telegraf
func (c *HccsCollector) UpdateTelegraf(fieldsMap map[string]map[string]interface{}, n *colcommon.NpuCollector,
	containerMap map[int32]container.DevicesInfo, chips []colcommon.HuaWeiAIChip) map[string]map[string]interface{} {

	caches := colcommon.GetInfoFromCache[hccsCache](n, colcommon.GetCacheKey(c))
	for _, chip := range chips {
		cache, ok := caches[chip.PhyId]
		if !ok {
			continue
		}
		fieldMap := getFieldMap(fieldsMap, cache.chip.LogicID)

		telegrafUpdateHccsStatisticInfo(cache, c, fieldMap)
		telegrafUpdateHccsBwInfo(cache, c, fieldMap)
	}

	return fieldsMap

}

func telegrafUpdateHccsBwInfo(cache hccsCache, c *HccsCollector, fieldMap map[string]interface{}) {
	bandwidthInfo := cache.hccsBW
	if bandwidthInfo == nil || c.hccsBeginIndex < 0 {
		return
	}
	for i := c.hccsBeginIndex; i < MaxHccsNum; i++ {
		doUpdateTelegraf(fieldMap, hccsBWTxDescs[i], bandwidthInfo.TxBandwidth[i], "")
		doUpdateTelegraf(fieldMap, hccsBWRxDescs[i], bandwidthInfo.RxBandwidth[i], "")
	}
	doUpdateTelegraf(fieldMap, hccsBWProfilingTime, bandwidthInfo.ProfilingTime, "")
	doUpdateTelegraf(fieldMap, hccsBWTotalTx, bandwidthInfo.TotalTxbw, "")
	doUpdateTelegraf(fieldMap, hccsBWTotalRx, bandwidthInfo.TotalRxbw, "")
}

func telegrafUpdateHccsStatisticInfo(cache hccsCache, c *HccsCollector, fieldMap map[string]interface{}) {
	statisticInfo := cache.hccsStat

	if statisticInfo == nil || c.hccsBeginIndex < 0 {
		return
	}
	for i := c.hccsBeginIndex; i < MaxHccsNum; i++ {
		doUpdateTelegraf(fieldMap, hccsTxDescs[i], statisticInfo.TxCnt[i], "")
		doUpdateTelegraf(fieldMap, hccsRxDescs[i], statisticInfo.RxCnt[i], "")
		doUpdateTelegraf(fieldMap, hccsErrDescs[i], statisticInfo.CrcErrCnt[i], "")
	}
}
