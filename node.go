package main

import (
	"time"

	log "github.com/sirupsen/logrus"

	qlcsdk "github.com/qlcchain/qlc-go-sdk"
	"github.com/qlcchain/qlc-go-sdk/pkg/types"
	"github.com/qlcchain/qlc-go-sdk/pkg/util"
)

const (
	MaxGetWorkIntervalSec   = 60 * 10
	MaxGetHeaderIntervalSec = 10
)

type NodeClient struct {
	client       *qlcsdk.QLCClient
	minerAddrStr string
	minerAddr    types.Address
	algoName     string

	lastGetWorkRsp  *qlcsdk.PovApiGetWork
	lastGetWorkTime time.Time
	lastHeaderTime  time.Time

	getWorkOk     int
	getWorkErr    int
	submitWorkOk  int
	submitWorkErr int

	eventChan chan Event
	quitCh    chan struct{}
}

func NewNodeClient(url string, minerAddr string, algoName string) *NodeClient {
	var err error

	nc := new(NodeClient)
	nc.minerAddrStr = minerAddr
	nc.algoName = algoName
	nc.minerAddr, err = types.HexToAddress(minerAddr)
	if err != nil {
		log.Errorln(err)
		return nil
	}

	nc.client, err = qlcsdk.NewQLCClient(url)
	if err != nil {
		log.Errorln(err)
		return nil
	}

	nc.eventChan = make(chan Event, EventMaxChanSize)
	nc.quitCh = make(chan struct{})

	return nc
}

func (nc *NodeClient) Start() error {
	GetDefaultEventBus().Subscribe(EventJobSubmit, nc.eventChan)
	GetDefaultEventBus().Subscribe(EventStatisticsTicker, nc.eventChan)

	go nc.loop()

	return nil
}

func (nc *NodeClient) Stop() {
	close(nc.quitCh)
}

func (nc *NodeClient) loop() {
	log.Infof("node running loop, miner:%s, algo:%s", nc.minerAddrStr, nc.algoName)

	fetchTicker := time.NewTicker(5 * time.Second)
	defer fetchTicker.Stop()

	for {
		select {
		case <-nc.quitCh:
			return
		case <-fetchTicker.C:
			nc.fetchNewWork()
		case event := <-nc.eventChan:
			nc.consumeEvent(event)
		}
	}
}

func (nc *NodeClient) consumeEvent(event Event) {
	switch event.Topic {
	case EventJobSubmit:
		nc.consumeJobSubmit(event)
	case EventStatisticsTicker:
		nc.consumeStatisticsTicker(event)
	}
}

func (nc *NodeClient) consumeJobSubmit(event Event) {
	work := event.Data.(*JobWork)
	if len(work.submits) <= 0 {
		log.Errorf("submit not exist for job work %s", work.JobHash)
		return
	}
	js := work.submits[len(work.submits)-1]

	apiSubmit := new(qlcsdk.PovApiSubmitWork)

	apiSubmit.WorkHash = work.WorkHash
	apiSubmit.Nonce = js.Nonce
	apiSubmit.Timestamp = js.NTime
	apiSubmit.CoinbaseExtra = work.CoinbaseExtra
	apiSubmit.CoinbaseHash = work.CoinbaseHash
	apiSubmit.MerkleRoot = work.MerkleRoot
	apiSubmit.BlockHash = work.BlockHash

	nc.submitWork(apiSubmit)
}

func (nc *NodeClient) consumeStatisticsTicker(event Event) {
	log.Infof("node rpcs: getWorkOk:%d, getWorkErr:%d, submitWorkOk:%d, submitWorkErr:%d",
		nc.getWorkOk, nc.getWorkErr, nc.submitWorkOk, nc.submitWorkErr)
}

func (nc *NodeClient) fetchNewWork() {
	timeNow := time.Now()

	// check getLatestHeader is too fast
	if nc.lastHeaderTime.Add(MaxGetHeaderIntervalSec * time.Second).After(timeNow) {
		return
	}
	getHeaderRsp := nc.getLatestHeader()
	if getHeaderRsp == nil {
		return
	}
	nc.lastHeaderTime = timeNow

	// check getWork is too fast when parent block not changed
	if nc.lastGetWorkRsp != nil && nc.lastGetWorkRsp.Previous == getHeaderRsp.GetHash() {
		if nc.lastGetWorkTime.Add(MaxGetWorkIntervalSec * time.Second).After(timeNow) {
			return
		}
	}
	getWorkRsp := nc.getWork()
	if getWorkRsp == nil {
		return
	}
	nc.lastGetWorkTime = timeNow

	// check same work
	if nc.lastGetWorkRsp != nil && nc.lastGetWorkRsp.WorkHash == getWorkRsp.WorkHash {
		return
	}
	nc.lastGetWorkRsp = getWorkRsp

	GetDefaultEventBus().Publish(EventUpdateApiWork, getWorkRsp)
}

func (nc *NodeClient) getWork() *qlcsdk.PovApiGetWork {
	getWorkRsp, err := nc.client.Pov.GetWork(nc.minerAddr, nc.algoName)
	if err != nil {
		log.Errorln(err)
		nc.getWorkErr++
		return nil
	}
	nc.getWorkOk++

	log.Infof("getWork response: %s", util.ToString(getWorkRsp))

	return getWorkRsp
}

func (nc *NodeClient) submitWork(submitWorkReq *qlcsdk.PovApiSubmitWork) {
	log.Infof("submitWork request: %s", util.ToString(submitWorkReq))
	err := nc.client.Pov.SubmitWork(submitWorkReq)
	if err != nil {
		log.Errorln(err)
		nc.submitWorkErr++
		return
	}
	nc.submitWorkOk++
}

func (nc *NodeClient) getLatestHeader() *qlcsdk.PovApiHeader {
	getHeaderRsp, err := nc.client.Pov.GetLatestHeader()
	if err != nil {
		log.Errorln(err)
		return nil
	}

	log.Debugf("getLatestHeader response: %d/%s, ", getHeaderRsp.GetHeight(), getHeaderRsp.GetHash())

	return getHeaderRsp
}
