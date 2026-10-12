/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package parallel 并发处理，主要用于压测的场景
package parallel

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"chainmaker.org/chainmaker/logger/v2"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	sdk "chainmaker.org/chainmaker/sdk-go/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var flags *pflag.FlagSet

var log = logger.GetLogger(logger.MODULE_CLI)

// 用来接受cmd参数的变量
var (
	threadNum      int
	loopNum        int
	timeout        int
	printTime      int
	sleepTime      int
	climbTime      int
	requestTimeout int64
	runTime        int32
	checkResult    bool
	recordLog      bool
	showKey        bool
	useTLS         bool
	useShortCrt    bool

	hostsString           string
	hostnamesString       string
	statisticalType       string
	checkInterval         int
	onlySend              bool
	signCrtPathsString    string
	signKeyPathsString    string
	userCrtPathsString    string
	userKeyPathsString    string
	userEncKeyPathsString string
	userEncCrtPathsString string
	orgIDsString          string
	hashAlgo              string
	caPathsString         string
	pairsFile             string
	pairsString           string
	globalPairs           []*KeyValuePair
	abiPath               string
	method                string
	orgIds                string // 组织列表(多个用逗号隔开)
	adminSignKeys         string // 管理员私钥列表(多个用逗号隔开)
	adminSignCrts         string // 管理员证书列表(多个用逗号隔开)
	chainId               string
	contractName          string
	version               string
	wasmPath              string
	prepareBuild          bool
	caPaths               []string
	hosts                 []string
	hostnames             []string
	signCrtPaths          []string
	signKeyPaths          []string
	encKeyPaths           []string
	encCrtPaths           []string
	encCrtBytes           [][]byte
	encKeyBytes           [][]byte
	userCrtPaths          []string
	userKeyPaths          []string
	adminKeyPaths         []string
	adminCrtPaths         []string
	orgIDs                []string

	nodeNum int

	fileCache = NewFileCacheReader()
	certCache = NewCertFileCacheReader()

	abiCache     = NewFileCacheReader()
	outputResult bool

	authTypeUint32 uint32
	authType       sdk.AuthType
	gasLimit       uint64

	startBlock int64 // 订阅的起始区块高度
	endBlock   int64 // 订阅的结束区块高度
)

// 用来定义cmd中flag常量
const (
	threadNumFlag             = "thread-num"
	loopNumFlag               = "loop-num"
	timeoutFlag               = "timeout"
	printTimeFlag             = "print-time"
	sleepTimeFlag             = "sleep-time"
	climbTimeFlag             = "climb-time"
	hostsStringFlag           = "hosts"
	signCrtPathsStringFlag    = "sign-crts"
	signKeyPathsStringFlag    = "sign-keys"
	userCrtPathsStringFlag    = "user-crts"
	userKeyPathsStringFlag    = "user-keys"
	userEncKeyPathsStringFlag = "user-enc-keys"
	userEncCrtPathsStringFlag = "user-enc-crts"
	orgIDsStringFlag          = "org-IDs"
	checkResultFlag           = "check-result"
	recordLogFlag             = "record-log"
	outputResultFlag          = "output-result"
	showKeyFlag               = "show-key"
	hashAlgoFlag              = "hash-algorithm"
	caPathsStringFlag         = "ca-path"
	useTLSFlag                = "use-tls"
	orgIdsFlag                = "org-ids"
	adminSignKeysFlag         = "admin-sign-keys"
	adminSignCrtsFlag         = "admin-sign-crts"
	chainIdFlag               = "chain-id"
	contractNameFlag          = "contract-name"
	useShortCrtFlag           = "use-short-crt"
	requestTimeoutFlag        = "request-timeout"
	authTypeUint32Flag        = "auth-type"
	gasLimitFlag              = "gas-limit"
	hostnamesStringFlag       = "tls-host-names"
	checkIntervalFlag         = "check-interval"
	onlySendFlag              = "only-send"
	pairsStringFlag           = "pairs"
	pairsFileFlag             = "pairs-file"
	methodFlag                = "method"
	abiPathFlag               = "abi-path"
	wasmPathFlag              = "wasm-path"
	runTimeFlag               = "run-time"
	versionFlag               = "version"
	prepareBuildFlag          = "prepare-build"
)

// 用来控制pairs参数的常量
const (
	LoopTypeEnd     = int8(1)
	LoopTypeRestart = int8(2)
)

// ValueParam pairs参数控制接口题
type ValueParam struct {
	Initial      int64 `json:"initial"`
	Increase     bool  `json:"increase"`
	EndValue     int64 `json:"endValue"`
	TempIntValue int64 `json:"-"`
	LoopType     int8  `json:"loopType"`
}

// KeyValuePair pairs参数结构体
type KeyValuePair struct {
	Key        string `json:"key,omitempty"`
	Value      string `json:"value,omitempty"`
	Unique     bool   `json:"unique,omitempty"`
	RandomRate int64  `json:"randomRate,omitempty"`
	Increase   bool   `json:"increase"`
	Decrease   bool   `json:"decrease"`
	// mu protect IntValue in Increase/Decrease scene.
	mu          sync.Mutex
	IntValue    int64         `json:"-"`
	ValueFormat string        `json:"valueFormat,omitempty"`
	ValueParams []*ValueParam `json:"valueParams,omitempty"`
	EndCount    int           `json:"-"` // 需要终止的参数数量，如果全部需要终止则停止压测
	ArriveCount int           `json:"-"` // 已经达成目标值的数量,与ArriveArr互相配合，主要记录ArriveArr中为true的值
	ArriveArr   []bool        `json:"-"` // 判断每个valueParam是否达成条件值如果达成则设置为true
	Values      []int64       `json:"-"`
	IntPows     []int64       `json:"-"`
}

func init() {
	flags = &pflag.FlagSet{}
	flags.IntVarP(&threadNum, threadNumFlag, "N", 10, "specify thread number")
	flags.IntVarP(&loopNum, loopNumFlag, "l", 1000, "specify loop number")
	flags.IntVarP(&timeout, timeoutFlag, "T", 2, "specify timeout(unit: s)")
	flags.IntVarP(&printTime, printTimeFlag, "r", 1, "specify print time(unit: s)")
	flags.IntVarP(&sleepTime, sleepTimeFlag, "S", 100, "specify sleep time(unit: ms)")
	flags.IntVarP(&climbTime, climbTimeFlag, "L", 10, "specify climb time(unit: s)")
	flags.StringVarP(&hostsString, hostsStringFlag, "H", "", "specify hosts")
	flags.StringVarP(&signCrtPathsString, signCrtPathsStringFlag, "K", "", "specify user crt path")
	flags.StringVarP(&signKeyPathsString, signKeyPathsStringFlag, "u", "", "specify user key path")
	flags.StringVar(&userCrtPathsString, userCrtPathsStringFlag, "", "specify tls crt path")
	flags.StringVar(&userKeyPathsString, userKeyPathsStringFlag, "", "specify tls key path")
	flags.StringVar(&userEncKeyPathsString, userEncKeyPathsStringFlag, "", "enc key path")
	flags.StringVar(&userEncCrtPathsString, userEncCrtPathsStringFlag, "", "enc certificate path")
	flags.StringVarP(&orgIDsString, orgIDsStringFlag, "I", "", "specify user key path")
	flags.BoolVarP(&checkResult, checkResultFlag, "Y", false, "specify whether check result")
	flags.BoolVarP(&recordLog, recordLogFlag, "g", false, "specify whether record log")
	flags.BoolVarP(&outputResult, outputResultFlag, "", false, "output rpc result, eg: txid")
	flags.BoolVarP(&showKey, showKeyFlag, "", false, "bool")
	flags.StringVar(&hashAlgo, hashAlgoFlag, "SHA256", "hash algorithm set in chain configuration")
	flags.StringVarP(&caPathsString, caPathsStringFlag, "P", "", "specify ca path")
	flags.BoolVarP(&useTLS, useTLSFlag, "t", false, "specify whether use tls")
	flags.StringVar(&orgIds, orgIdsFlag, "", "orgIds of admin")
	flags.StringVar(&adminSignKeys, adminSignKeysFlag, "", "adminSignKeys of admin")
	flags.StringVar(&adminSignCrts, adminSignCrtsFlag, "", "adminSignCrts of admin")
	flags.StringVarP(&chainId, chainIdFlag, "C", "chain1", "specify chain id")
	flags.StringVarP(&contractName, contractNameFlag, "n", "", "specify contract name")
	flags.BoolVar(&useShortCrt, useShortCrtFlag, false, "use compressed certificate in transactions")
	flags.Int64Var(&requestTimeout, requestTimeoutFlag, 5, "specify request timeout(unit: s)")
	flags.Uint32Var(&authTypeUint32, authTypeUint32Flag, 1, "chainmaker auth type. PermissionedWithCert:1,PermissionedWithKey:2,Public:3")
	flags.Uint64Var(&gasLimit, gasLimitFlag, 0, "gas limit in uint64 type")
	flags.StringVarP(&hostnamesString, hostnamesStringFlag, "", "", "specify hostname, the sequence is the same as --hosts")
	flags.IntVarP(&checkInterval, checkIntervalFlag, "", 1, "After all threads are done,check the interval time of the last block generation. ")
	flags.BoolVarP(&onlySend, onlySendFlag, "", false, "The result statistics are open, and the result is true. Only RPC request data is counted, and the on chain results are not counted")
	// invoke
	flags.StringVarP(&pairsString, pairsStringFlag, "a", "[{\"key\":\"key\",\"value\":\"counter1\",\"unique\":false}]", "specify pairs")
	flags.StringVarP(&pairsFile, pairsFileFlag, "A", "", "specify pairs file, if used, set --pairs=\"\"")
	flags.StringVarP(&method, methodFlag, "m", "increase", "specify contract method")
	flags.StringVarP(&abiPath, abiPathFlag, "", "", "abi file path")
	// query
	//flags.StringVarP(&pairsString, "pairs", "a", "[{\"key\":\"key\",\"value\":\"counter1\",\"unique\":false}]", "specify pairs")
	//flags.StringVarP(&pairsFile, "pairs-file", "A", "", "specify pairs file, if used, set --pairs=\"\"")
	//flags.StringVarP(&method, "method", "m", "increase", "specify contract method")
	// upgrade
	flags.StringVarP(&wasmPath, wasmPathFlag, "w", "", "specify wasm path")
	flags.Int32VarP(&runTime, runTimeFlag, "R", int32(common.RuntimeType_GASM), "specify run time")
	flags.StringVarP(&version, versionFlag, "v", "", "specify contract version")
	flags.BoolVarP(&prepareBuild, prepareBuildFlag, "", false, "prepare build request param")
	// create
	//flags.StringVarP(&wasmPath, "wasm-path", "w", "", "specify wasm path")
	//flags.Int32VarP(&runTime, "run-time", "m", int32(common.RuntimeType_GASM), "specify run time")
}

// ParallelCMD parallel sub command
// @return *cobra.Command
func ParallelCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parallel",
		Short: "Parallel",
		Long:  "Parallel",
	}
	cmd.AddCommand(invokeCMD())
	cmd.AddCommand(queryCMD())
	cmd.AddCommand(createContractCMD())
	cmd.AddCommand(upgradeContractCMD())
	cmd.AddCommand(statCMD())
	return cmd
}

func paramRead() {
	authType = sdk.AuthType(authTypeUint32)
	// 这个是节点的ip
	if hostsString != "" {
		hosts = strings.Split(hostsString, ",")
	}
	nodeNum = len(hosts)
	// 之前起的名字有问题留到下个版本修改，这个是tls
	if hostnamesString != "" {
		hostnames = strings.Split(hostnamesString, ",")
	}
	if signKeyPathsString != "" {
		signKeyPaths = strings.Split(signKeyPathsString, ",")
	}
	if signCrtPathsString != "" {
		signCrtPaths = strings.Split(signCrtPathsString, ",")
	}
	if caPathsString != "" {
		caPaths = strings.Split(caPathsString, ",")
	}
	if userKeyPathsString != "" {
		userKeyPaths = strings.Split(userKeyPathsString, ",")
	}
	if userCrtPathsString != "" {
		userCrtPaths = strings.Split(userCrtPathsString, ",")
	}
	if adminSignKeys != "" {
		adminKeyPaths = strings.Split(adminSignKeys, ",")
	}
	if adminSignCrts != "" {
		adminCrtPaths = strings.Split(adminSignCrts, ",")
	}
	if userEncKeyPathsString != "" {
		encKeyPaths = strings.Split(userEncKeyPathsString, ",")
	}
	if userEncCrtPathsString != "" {
		encCrtPaths = strings.Split(userEncCrtPathsString, ",")
	}
	if orgIDsString != "" {
		orgIDs = strings.Split(orgIDsString, ",")
	}
	// 读取pair文件
	if len(pairsFile) != 0 {
		bytes, err := ioutil.ReadFile(pairsFile)
		if err != nil {
			panic(err)
		}
		pairsString = string(bytes)
	}
	var err error
	globalPairs, err = getPairInfos()
	if err != nil {
		panic(err)
	}
}

// paramCheck 参数校验
func paramCheck() error {
	// 解析host
	if hostsString == "" {
		return fmt.Errorf("hosts is required")
	}
	// 校验不同认证模式下的证书
	authType = sdk.AuthType(authTypeUint32)
	switch authType {
	case sdk.Public:
		if err := pkCheck(); err != nil {
			return err
		}
	case sdk.PermissionedWithCert:
		if err := certCheck(); err != nil {
			return err
		}
	}
	// 先做tls校验
	if err := tlsCheck(); err != nil {
		return err
	}
	// 再做双证书校验
	// 先做tls校验
	if err := twoWayCheck(); err != nil {
		return err
	}
	return nil
}

// pk模式的参数校验
func pkCheck() error {
	if len(signKeyPaths) == 0 {
		return fmt.Errorf("pk mode need input sign key paths")
	}
	// 如果填写多个signKey那么必须和节点数量保持一致
	if len(hosts) > 1 && len(signKeyPaths) > 1 {
		if len(hosts) != len(signKeyPaths) {
			return fmt.Errorf("input multiple host names , but host number not equals "+
				"sign-keys number [%d|%d]", len(hosts), len(signKeyPaths))
		}
	}
	if useTLS {
		// tlsCheck 方法会校验 user-key 与 user-crt是否与节点数量一致，所以这里只对比4个证书的数量是否一致
		if !equal(len(signKeyPaths), len(userKeyPaths), len(userCrtPaths)) {
			return fmt.Errorf("sign key , user key, user crt not equals num[%d|%d|%d]",
				len(signKeyPaths), len(userKeyPaths), len(userCrtPaths))
		}
		if len(encKeyPaths) > 1 || len(encCrtPaths) > 1 {
			if !equal(len(signKeyPaths), len(encKeyPaths), len(encCrtPaths)) {
				return fmt.Errorf("sign key , enc key, enc crt not equals num[%d|%d|%d]",
					len(signKeyPaths), len(encKeyPaths), len(encCrtPaths))
			}
		}
	}
	return nil
}

// cert模式的参数校验
func certCheck() error {
	if len(signKeyPaths) == 0 {
		return fmt.Errorf("cert mode neer input sign-keys")
	}
	if len(signCrtPaths) == 0 {
		return fmt.Errorf("cert mode neer input sign-crts")
	}
	if len(orgIDs) == 0 {
		return fmt.Errorf("cert mode need input org-IDs")
	}
	// 如果填写了多个key或crt那么key与crt的数量必须要和节点的数量保持一致
	if len(hosts) > 1 && (len(signKeyPaths) > 1 || len(signCrtPaths) > 1) {
		if len(hosts) != len(signKeyPaths) {
			return fmt.Errorf("input multiple host names , but host number not equals "+
				"sign-keys number [%d|%d]", len(hosts), len(signKeyPaths))
		}
		if len(hosts) != len(signCrtPaths) {
			return fmt.Errorf("input multiple host names , but host number not equals "+
				"sign-crts number [%d|%d]", len(hosts), len(signCrtPaths))
		}
		if len(hosts) != len(orgIDs) {
			return fmt.Errorf("input multiple host names , but host number not equals "+
				"org-IDs number [%d|%d]", len(hosts), len(orgIDs))
		}
	}
	// tlsCheck 方法会校验 user-key 与 user-crt是否与节点数量一致，所以这里只对比4个证书的数量是否一致
	if useTLS {
		if !equal(len(signKeyPaths), len(signCrtPaths), len(userKeyPaths), len(userCrtPaths)) {
			return fmt.Errorf("sign key ,sign crt , user key, user crt not equals num[%d|%d|%d|%d]",
				len(signKeyPaths), len(signCrtPaths), len(userKeyPaths), len(userCrtPaths))
		}
		// 如果开启了国密就校验国密的证书数量一致性
		if len(encKeyPaths) > 1 || len(encCrtPaths) > 1 {
			if !equal(len(signKeyPaths), len(signCrtPaths), len(encKeyPaths), len(encCrtPaths)) {
				return fmt.Errorf("sign key ,sign crt , enc key, enc crt not equals num[%d|%d|%d|%d]",
					len(signKeyPaths), len(signCrtPaths), len(encKeyPaths), len(encCrtPaths))
			}
		}
	}
	return nil
}

// tls校验当，开启tls时需要做的一些必要的验证
func tlsCheck() error {
	if !useTLS {
		return nil
	}
	if len(caPaths) == 0 {
		return fmt.Errorf("tls is true need input ca-paths")
	}
	// 这里hostname是tls host
	if len(hostnames) == 0 {
		return fmt.Errorf("tls is true need input hostnames")
	}
	if len(userKeyPaths) == 0 || len(userCrtPaths) == 0 {
		return fmt.Errorf("no user cert path or no user key path")
	}
	// tls host要么填1个要么和节点的数量保持一致
	if len(hostnames) > 1 && len(hosts) != len(hostnames) {
		return fmt.Errorf("input multiple host names , but host number not equals "+
			"hostnames number [%d|%d]", len(hosts), len(hostnames))
	}
	return nil
}

// endorserCheck 如果创建或者升级合约则需要校验背书信息
func endorserCheck() error {
	if len(adminKeyPaths) == 0 {
		return fmt.Errorf("endorser is true need input admin key paths")
	}
	if authType != sdk.Public && len(adminCrtPaths) == 0 {
		return fmt.Errorf("endorser is true need input admin crt paths")
	}
	return nil
}

// 双证书校验
func twoWayCheck() error {
	if !useTLS {
		return nil
	}
	if len(encCrtPaths) == 0 && len(encKeyPaths) == 0 {
		return nil
	}
	if len(encCrtPaths) != 0 && len(encKeyPaths) == 0 {
		return fmt.Errorf("use two way input user-enc-crts but not input user-enc-keys")
	}
	if len(encCrtPaths) == 0 && len(encKeyPaths) != 0 {
		return fmt.Errorf("use two way input user-enc-keys but not input user-enc-crts")
	}
	return nil
}

func intPow(base, exp int64) int64 {
	result := int64(1)
	for i := int64(0); i < exp; i++ {
		result *= base
	}
	return result
}

func getPairInfos() ([]*KeyValuePair, error) {
	var ps []*KeyValuePair
	err := json.Unmarshal([]byte(pairsString), &ps)
	if err != nil {
		log.Errorf("unmarshal pair content failed, origin content: %s, err: %s", pairsString, err)
		return nil, err
	}
	for _, p := range ps {
		if p.Decrease || p.Increase {
			p.IntValue, err = strconv.ParseInt(p.Value, 10, 64)
			if err != nil {
				return nil, err
			}
		}
		if p.ValueFormat != "" {
			re := regexp.MustCompile(`%0(\d+)d`)
			matches := re.FindAllStringSubmatch(p.ValueFormat, -1)
			if len(matches) != len(p.ValueParams) {
				return nil, fmt.Errorf("not enough (or more) values to fill the value format template")
			}
			p.Values = make([]int64, len(p.ValueParams))
			p.IntPows = make([]int64, len(p.ValueParams))
			for i, match := range matches {
				if p.ValueParams[i].LoopType == LoopTypeEnd {
					p.EndCount++
				}
				p.ArriveArr = make([]bool, len(p.ValueParams))
				p.Values[i] = p.ValueParams[i].Initial
				p.ValueParams[i].TempIntValue = p.Values[i]
				width, _ := strconv.Atoi(match[1])
				p.IntPows[i] = intPow(10, int64(width))
			}
		}
	}

	return ps, nil
}

func equal(nums ...int) bool {
	if len(nums) == 0 {
		return true // 无元素视为相等，或根据需求返回 false
	}
	first := nums[0]
	for _, num := range nums[1:] {
		if num != first {
			return false
		}
	}
	return true
}
