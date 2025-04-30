/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2021-2024. All rights reserved.
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

// Package common for general collector
package common

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"

	"ascend-common/common-utils/hwlog"
	"ascend-common/devmanager"
	"ascend-common/devmanager/common"
	"huawei.com/npu-exporter/v6/collector/container"
	"huawei.com/npu-exporter/v6/collector/container/isula"
	"huawei.com/npu-exporter/v6/collector/container/v1"
	"huawei.com/npu-exporter/v6/utils/logger"
)

var (
	defaultLogFile = "/var/log/mindx-dl/npu-exporter/npu-exporter.log"
	mockErr        = errors.New("mockErr")
)

const (
	cacheTime         = 60 * time.Second
	timestamp         = 1606402
	waitTime          = 2 * time.Second
	npuCount          = 8
	time5s            = 5 * time.Second
	defaultUpdateTime = 10 * time.Millisecond
	num2              = 2
	num100            = 100
)

type mockContainerRuntimeOperator struct{}

// Init implements ContainerRuntimeOperator
func (operator *mockContainerRuntimeOperator) Init() error {
	return nil
}

// Close implements ContainerRuntimeOperator
func (operator *mockContainerRuntimeOperator) Close() error {
	return nil
}

// ContainerIDs implements ContainerRuntimeOperator
func (operator *mockContainerRuntimeOperator) GetContainers(ctx context.Context) ([]*container.CommonContainer, error) {
	return []*container.CommonContainer{}, nil
}

// GetContainerInfoByID implements ContainerRuntimeOperator
func (operator *mockContainerRuntimeOperator) GetContainerInfoByID(ctx context.Context, id string) (v1.Spec, error) {
	return v1.Spec{}, nil
}

// GetIsulaContainerInfoByID implements ContainerRuntimeOperator
func (operator *mockContainerRuntimeOperator) GetIsulaContainerInfoByID(ctx context.Context,
	id string) (isula.ContainerJson, error) {
	return isula.ContainerJson{}, nil
}

// GetContainerType implements ContainerRuntimeOperator
func (operator *mockContainerRuntimeOperator) GetContainerType() string {
	return container.DefaultContainer
}

func mockScan4AscendDevices(_ string) ([]int, bool, error) {
	return []int{1}, true, nil
}

func mockGetCgroupPath(controller, specCgroupsPath string) (string, error) {
	return "", nil
}

func makeMockDevicesParser() *container.DevicesParser {
	return &container.DevicesParser{
		RuntimeOperator: new(mockContainerRuntimeOperator),
	}
}

type newNpuCollectorTestCase struct {
	cacheTime    time.Duration
	updateTime   time.Duration
	deviceParser *container.DevicesParser
	dmgr         *devmanager.DeviceManager
}

// TestNewNpuCollector test method of NewNpuCollector
func TestNewNpuCollector(t *testing.T) {
	tc := newNpuCollectorTestCase{
		cacheTime:    cacheTime,
		updateTime:   defaultUpdateTime,
		deviceParser: &container.DevicesParser{},
		dmgr:         &devmanager.DeviceManager{},
	}

	c := NewNpuCollector(tc.cacheTime, tc.updateTime, tc.deviceParser, tc.dmgr)

	assert.NotNil(t, c)
}

type testCase struct {
	name        string
	wantErr     bool
	mockPart    interface{}
	expectValue interface{}
	expectCount interface{}
}

func newTestCase(name string, wantErr bool, mockPart interface{}) testCase {
	return testCase{
		name:     name,
		wantErr:  wantErr,
		mockPart: mockPart,
	}
}

// TestGetChipInfo test  method getChipInfo
func TestGetChipInfo(t *testing.T) {
	tests := []testCase{
		newTestCase("should return chip info successfully when dsmi works normally", false,
			&devmanager.DeviceManagerMock{}),
		newTestCase("should return nil when dsmi works abnormally", true, &devmanager.DeviceManagerMockErr{}),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chipInfo := getNPUChipList(tt.mockPart.(devmanager.DeviceInterface))
			t.Logf("%#v", chipInfo)
			assert.NotNil(t, chipInfo)
			if tt.wantErr {
				assert.Len(t, chipInfo, 0)
			} else {
				assert.NotNil(t, chipInfo)
			}
		})
	}
}

func init() {
	logger.HwLogConfig = &hwlog.LogConfig{
		OnlyToStdout: true,
	}
	logger.InitLogger("Prometheus")
}

func mockGetNPUChipList() []HuaWeiAIChip {
	chips := make([]HuaWeiAIChip, 0)
	for id := int32(0); id < npuCount; id++ {
		chip := HuaWeiAIChip{
			CardId:   id,
			PhyId:    id,
			DeviceID: id,
			LogicID:  id,
		}

		chips = append(chips, chip)
	}
	return chips
}

// TestInitCardInfo test  method getChipInfo
func TestInitCardInfo(t *testing.T) {
	patches := gomonkey.ApplyFuncReturn(getNPUChipList, mockGetNPUChipList())
	defer patches.Reset()
	convey.Convey("test InitCardInfo", t, func() {

		ctx, cancelFunc := context.WithCancel(context.Background())
		defer cancelFunc()
		npuCollector := mockNewNpuCollector()

		InitCardInfo(&sync.WaitGroup{}, ctx, npuCollector)
		time.Sleep(time.Millisecond * num100)
		cancelFunc()
		chips := getChipListCache(npuCollector)
		convey.So(len(chips), convey.ShouldEqual, npuCount)
	})
}

// TestGetChipListCache test  method getChipListCache
func TestGetChipListCache(t *testing.T) {
	npuCollector := mockNewNpuCollector()
	tests := []testCase{
		{name: "should return 0 chips when cache is nil", wantErr: false, mockPart: func() {}, expectCount: 0},
		{name: "should return chips : " + strconv.Itoa(npuCount), expectCount: npuCount, wantErr: false,
			mockPart: func() { npuCollector.cache.Set(npuListCacheKey, mockGetNPUChipList(), cacheTime) }},
		{name: "should return 0 chips when cache value is nil", wantErr: false, expectCount: 0,
			mockPart: func() { npuCollector.cache.Set(npuListCacheKey, nil, cacheTime) }},
		{name: "should return 0 chips when value is a incorrect type", expectCount: 0, wantErr: false,
			mockPart: func() { npuCollector.cache.Set(npuListCacheKey, &HuaWeiAIChip{}, cacheTime) }},
		{name: "should return 0 chips when cache is empty", expectCount: 0, wantErr: false,
			mockPart: func() { npuCollector.cache.Set(npuListCacheKey, []HuaWeiAIChip{}, cacheTime) },
		},
	}

	convey.Convey("getChipListCache", t, func() {
		for _, tt := range tests {
			convey.Convey(tt.name, func() {
				tt.mockPart.(func())()
				chips := getChipListCache(npuCollector)
				assert.Len(t, chips, tt.expectCount.(int))
				convey.So(len(chips), convey.ShouldEqual, tt.expectCount)
			})
		}
	})
}

func mockNewNpuCollector() *NpuCollector {
	tc := newNpuCollectorTestCase{
		cacheTime:    cacheTime,
		updateTime:   defaultUpdateTime,
		deviceParser: &container.DevicesParser{},
		dmgr:         &devmanager.DeviceManager{},
	}
	c := NewNpuCollector(tc.cacheTime, tc.updateTime, tc.deviceParser, tc.dmgr)
	return c
}

func TestNpuChipInfoInitAtFirstTime(t *testing.T) {
	n := mockNewNpuCollector()
	convey.Convey("TestNpuChipInfoInitAtFirstTime", t, func() {
		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyFuncReturn(getNPUChipList, []HuaWeiAIChip{{CardId: 0}})
		// do test
		npuChipInfoInitAtFirstTime(n)
		// valid cache
		data, err := n.cache.Get(npuListCacheKey)
		convey.So(err, convey.ShouldBeNil)
		chips, ok := data.([]HuaWeiAIChip)
		convey.So(ok, convey.ShouldBeTrue)
		convey.So(len(chips), convey.ShouldEqual, 1)
	})
}

func TestStartCollectSingleGoroutine(t *testing.T) {
	const mockKey = "mockKey"
	const mockValue = "mockValue"
	n := mockNewNpuCollector()
	wg := sync.WaitGroup{}
	ChainForSingleGoroutine = []MetricsCollector{
		&MetricsCollectorAdapter{},
	}
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethod(&MetricsCollectorAdapter{}, "CollectToCache",
		func(_ *MetricsCollectorAdapter, n *NpuCollector, chipList []HuaWeiAIChip) {
			n.cache.Set(mockKey, mockValue, n.cacheTime)
		})
	convey.Convey("TestStartCollectSingleGoroutine", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		startCollectSingleGoroutine(&wg, ctx, n, "")
		time.Sleep(n.updateTime)
		cancel()
		// valid cache
		data, err := n.cache.Get(mockKey)
		convey.So(err, convey.ShouldBeNil)
		value, ok := data.(string)
		convey.So(ok, convey.ShouldBeTrue)
		convey.So(value, convey.ShouldEqual, mockValue)
	})
}

type chipsCase struct {
	name        string
	devType     string
	buildChips  func()
	expectValue int
}

func TestGetChipListWithVNPU(t *testing.T) {
	n := mockNewNpuCollector()
	chip := HuaWeiAIChip{}
	tests := []chipsCase{
		{name: "TestGetChipListWithVNPU_310p_no_vnpu",
			devType: common.Ascend310P,
			buildChips: func() {
				chip = createChip()
			},
			expectValue: 1,
		},
		{name: "TestGetChipListWithVNPU_310p_2_vnpus",
			devType: common.Ascend310P,
			buildChips: func() {
				chip = createValidVnpuChip()
			},
			expectValue: num2,
		},
		{name: "TestGetChipListWithVNPU_910",
			devType: common.Ascend910,
			buildChips: func() {
				chip = createChip()
			},
			expectValue: 1,
		},
	}

	convey.Convey("TestGetChipListWithVNPU", t, func() {
		for _, tt := range tests {
			convey.Convey(tt.name, func() {
				tt.buildChips()
				patches := gomonkey.NewPatches()
				defer patches.Reset()
				patches.ApplyMethodReturn(n.Dmgr, "GetDevType", tt.devType)
				patches.ApplyFuncReturn(getChipListCache, []HuaWeiAIChip{chip})

				chips := GetChipListWithVNPU(n)
				convey.So(len(chips), convey.ShouldEqual, tt.expectValue)
			})
		}
	})
}

func createValidVnpuChip() HuaWeiAIChip {
	chip := createChip()
	chip.VDevInfos = &common.VirtualDevInfo{
		VDevActivityInfo: []common.VDevActivityInfo{
			{
				VDevID:       0,
				VDevAiCore:   0,
				VDevTotalMem: 0,
				VDevUsedMem:  0,
				IsVirtualDev: true,
			},
			{
				VDevID:       1,
				VDevAiCore:   1,
				VDevTotalMem: 1,
				VDevUsedMem:  1,
				IsVirtualDev: true,
			},
		},
	}
	return chip
}

func createChip() HuaWeiAIChip {
	return HuaWeiAIChip{
		CardId:   0,
		PhyId:    0,
		DeviceID: 0,
		LogicID:  0,
		ChipInfo: &common.ChipInfo{
			Name:    common.Ascend910,
			Type:    "Ascend",
			Version: "V1",
		},
	}
}

func TestSetPCIeBusInfo(t *testing.T) {
	const mockPcieBus = "0000:01:00.0"
	tests := []struct {
		name         string
		productTypes []string
		err          error
		expectValue  string
	}{{
		name:         "TestSetPCIeBusInfo_910",
		productTypes: []string{common.Ascend910},
		err:          nil,
		expectValue:  mockPcieBus,
	}, {
		name:         "TestSetPCIeBusInfo_910_err",
		productTypes: []string{common.Ascend910},
		err:          mockErr,
		expectValue:  "",
	}, {
		name:         "TestSetPCIeBusInfo_Atlas200ISoc",
		productTypes: []string{common.Atlas200ISoc},
		err:          nil,
		expectValue:  mockPcieBus,
	}, {
		name:         "TestSetPCIeBusInfo_Atlas200ISoc_err",
		productTypes: []string{common.Atlas200ISoc},
		err:          mockErr,
		expectValue:  "",
	}}
	chip := createChip()
	convey.Convey("TestSetPCIeBusInfo", t, func() {
		for _, tt := range tests {
			convey.Convey(tt.name, func() {
				dmgr := &devmanager.DeviceManager{ProductTypes: tt.productTypes}
				patches := gomonkey.NewPatches()
				defer patches.Reset()
				patches.ApplyMethodReturn(dmgr, "GetPCIeBusInfo", mockPcieBus, tt.err)

				setPCIeBusInfo(0, dmgr, &chip)
				convey.So(chip.PCIeBusInfo, convey.ShouldEqual, tt.expectValue)
			})
		}
	})
}
