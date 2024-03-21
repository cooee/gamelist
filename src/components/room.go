package core

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"gs/implement"
	"gs/implement/config"
	"gs/implement/types"
	pb "gs/protocol"

	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

	"github.com/itmisx/timewheel"
)

type Bet struct {
	Type  []pb.AreaType
	Bet   float64
	Round int //下注的轮数，方便undo

}

type BetInfo []*Bet

// =======================================================================
// var gmNumList []pb.AreaType = make([]pb.AreaType, 0) //gm提供骰子的结果
// var gmLock sync.Mutex = sync.Mutex{}

// var gmExtraPay [][]pb.AreaType = nil //gm暴击的数据
// var gmExtraLock sync.Mutex = sync.Mutex{}

// =======================================================================

type Room struct {
	/* 基本資料 */
	count      uint32 // 人數
	status     RoomStatus
	isInStatus bool //是否处于状态循环， 用户准备后，才会出发状态变化

	// map
	players map[uint32]*Player // 用戶清單

	//gm提供骰子的结果, 以playid为key
	gmNumMap map[uint32][]pb.AreaType

	//gm暴击的数据, 以playid为key
	gmExtraPay map[uint32][][]pb.AreaType

	betMap map[uint32]BetInfo //玩家下注的信息

	history [][]pb.AreaType

	/* channel */
	join    chan *Player  // 進入房間佇列
	command chan *Command // 用戶命令佇列
	close   chan int      // 關閉命令
	timeout chan *TimeOut //定时器事件

	roundIndex        int64 //牌局号
	needNewRoundIndex bool  //是否需要重新牌局号

	tw *timewheel.TimeWheel //时间轮

	roomStateTime time.Time       //状态开始时间
	gameReady     map[uint32]bool //游戏准备
}

// Command : 轉送封包資料
type Command struct {
	t      pb.U2S
	data   []byte
	player *Player
}

// 定时器事件
type TimeOut struct {
	data interface{}
}

// RoomStatus 房間狀態
type RoomStatus uint32

const (
	RoomStatusIdle   RoomStatus = iota // 閒置
	RoomStatusBet    RoomStatus = 1    //下注阶段
	RoomStatusSettle RoomStatus = 2    //结算阶段
)

const ROOM_STATE_TW_KEY = "ROOM_STATE_TW_KEY"

// 双符号下注的可能
var doubleBetArea = [][]pb.AreaType{
	{pb.AreaType_one, pb.AreaType_two},
	{pb.AreaType_one, pb.AreaType_three},
	{pb.AreaType_one, pb.AreaType_four},
	{pb.AreaType_one, pb.AreaType_five},
	{pb.AreaType_one, pb.AreaType_six},
	{pb.AreaType_two, pb.AreaType_three},
	{pb.AreaType_two, pb.AreaType_four},
	{pb.AreaType_two, pb.AreaType_five},
	{pb.AreaType_two, pb.AreaType_six},
	{pb.AreaType_three, pb.AreaType_four},
	{pb.AreaType_three, pb.AreaType_five},
	{pb.AreaType_three, pb.AreaType_six},
	{pb.AreaType_four, pb.AreaType_five},
	{pb.AreaType_four, pb.AreaType_six},
	{pb.AreaType_five, pb.AreaType_six},
}

// 保存进single里的索引
var singleRoundTargetIndex = map[int32]int{
	int32(pb.AreaType_one):   0,
	int32(pb.AreaType_two):   1,
	int32(pb.AreaType_three): 2,
	int32(pb.AreaType_four):  3,
	int32(pb.AreaType_five):  4,
	int32(pb.AreaType_six):   5,

	(int32(pb.AreaType_one)+1)*10 + int32(pb.AreaType_two):    6,
	(int32(pb.AreaType_one)+1)*10 + int32(pb.AreaType_three):  7,
	(int32(pb.AreaType_one)+1)*10 + int32(pb.AreaType_four):   8,
	(int32(pb.AreaType_one)+1)*10 + int32(pb.AreaType_five):   9,
	(int32(pb.AreaType_one)+1)*10 + int32(pb.AreaType_six):    10,
	(int32(pb.AreaType_two)+1)*10 + int32(pb.AreaType_three):  11,
	(int32(pb.AreaType_two)+1)*10 + int32(pb.AreaType_four):   12,
	(int32(pb.AreaType_two)+1)*10 + int32(pb.AreaType_five):   13,
	(int32(pb.AreaType_two)+1)*10 + int32(pb.AreaType_six):    14,
	(int32(pb.AreaType_three)+1)*10 + int32(pb.AreaType_four): 15,
	(int32(pb.AreaType_three)+1)*10 + int32(pb.AreaType_five): 16,
	(int32(pb.AreaType_three)+1)*10 + int32(pb.AreaType_six):  17,
	(int32(pb.AreaType_four)+1)*10 + int32(pb.AreaType_five):  18,
	(int32(pb.AreaType_four)+1)*10 + int32(pb.AreaType_six):   19,
	(int32(pb.AreaType_five)+1)*10 + int32(pb.AreaType_six):   20,
}

//------------------------------------------------------------------------------
//	create
//------------------------------------------------------------------------------

func CreateRoom() *Room {
	r := &Room{
		join:       make(chan *Player),
		command:    make(chan *Command, 100),
		timeout:    make(chan *TimeOut, 10),
		close:      make(chan int, 1),
		status:     RoomStatusIdle,
		isInStatus: false,

		gmNumMap:   make(map[uint32][]pb.AreaType),
		gmExtraPay: make(map[uint32][][]pb.AreaType),

		players: map[uint32]*Player{},
		history: make([][]pb.AreaType, 0),

		needNewRoundIndex: true,
		gameReady:         make(map[uint32]bool),

		betMap: make(map[uint32]BetInfo),
	}

	r.initHistory()

	r.tw = timewheel.New(time.Second, 60, r.twCallBack)
	go r.process()

	return r
}

//------------------------------------------------------------------------------
//	command
//------------------------------------------------------------------------------

// Command 命令分發中心
func (r *Room) Command(c *Command) {
	switch c.t {
	case pb.U2S_SPIN_REQ:
		r.Spin(c.player, c.data)
	case pb.U2S_GM_REQ:
		r.GM(c.player, c.data)
	case pb.U2S_GM_EXTRA_REQ:
		r.GMExtra(c.player, c.data)
	case pb.U2S_ROOM_INFO:
		r.roomInfo(c.player, c.data)
	case pb.U2S_HISTORY_REQ:
		r.onHistory(c.player, c.data)
	case pb.U2S_GAME_READY_REQ:
		r.onReady(c.player, c.data)
	case pb.U2S_CLEAR_REQ:
		r.onClear(c.player, c.data)
	case pb.U2S_UNDO_REQ:
		r.onUndo(c.player, c.data)
	case pb.U2S_CHANGE_ROOM_REQ:
		r.onChangeLevel(c.player, c.data)
	}
}

// Spin : 下注流程
func (r *Room) Spin(p *Player, data []byte) {
	req := &pb.SpinReq{}
	proto.Unmarshal(data, req)

	rsp := &pb.SpinResp{Ret: pb.ErroCode_Success, Detail: req.Detail}

	// 不属于下注阶段
	if r.status != RoomStatusBet {
		rsp.Ret = pb.ErroCode_NoOpGameState
		p.Send(pb.S2U_SPIN_ACK, rsp)
		return
	}

	// 玩家没有下注数据，发了一个空的数据过来
	if len(req.Detail) <= 0 {
		rsp.Ret = pb.ErroCode_BetNumError
		p.Send(pb.S2U_SPIN_ACK, rsp)
		return
	}

	// 计算一下下注的总额
	var totalBet float64 = 0
	for i := 0; i < len(req.Detail); i++ {
		// 下注为负数的时候，直接返回错误
		if req.Detail[i].Bet <= 0 {
			// 发送结果
			rsp.Ret = pb.ErroCode_BetNumError
			p.Send(pb.S2U_SPIN_ACK, rsp)
			return
		}

		// 判断提交了符合数量的符号
		betSymbolNum := len(req.Detail[i].Type)
		if betSymbolNum <= 0 || betSymbolNum > config.MaxSymbolNum {
			rsp.Ret = pb.ErroCode_IllegaRequest
			p.Send(pb.S2U_SPIN_ACK, rsp)
			return
		}

		// 判断是否提交了重复的符号
		if betSymbolNum > 1 {
			seen := make(map[pb.AreaType]bool)

			for _, v := range req.Detail[i].Type {
				if seen[v] {
					rsp.Ret = pb.ErroCode_IllegaRequest
					p.Send(pb.S2U_SPIN_ACK, rsp)
					return
				}
				seen[v] = true
			}
		}

		// 下注信息
		totalBet += req.Detail[i].Bet
	}

	// 算上已经押注的
	if _, ok := r.betMap[p.AccountId()]; ok {
		totalBet += r.getTotalBetMoney(r.betMap[p.AccountId()])
	}

	// 金币不足
	if totalBet > p.GetCoin() {
		rsp.Ret = pb.ErroCode_NoEnoughMoney
		// 发送结果
		p.Send(pb.S2U_SPIN_ACK, rsp)
		return
	}
	if _, ok := r.betMap[p.AccountId()]; !ok {
		r.betMap[p.AccountId()] = make(BetInfo, 0)
	}

	//================判断是否下注超上限=============================
	checkMaxBetlist := make(BetInfo, 0)
	betList := r.betMap[p.AccountId()]
	for _, v := range req.Detail {
		for _, v2 := range betList {
			if r.isSameAreaTypeList(v.Type, v2.Type) {
				checkMaxBetlist = append(checkMaxBetlist, &Bet{
					Bet:  v2.Bet,
					Type: v2.Type,
				})
			}
		}

		checkMaxBetlist = append(checkMaxBetlist, &Bet{
			Bet:  v.Bet,
			Type: v.Type,
		})
	}

	checkMaxBetlist = r.mergeBetInfos(checkMaxBetlist)
	for _, v := range checkMaxBetlist {
		if 1 == len(v.Type) {
			if v.Bet > config.SingleMaxBetMoney*p.user.Ratio() {
				rsp.Ret = pb.ErroCode_MaxBetMoney
				p.Send(pb.S2U_SPIN_ACK, rsp)
				return
			}
		}
	}
	//=============================================
	round := 0
	if len(r.betMap[p.AccountId()]) > 0 {
		round = r.betMap[p.AccountId()][len(r.betMap[p.AccountId()])-1].Round + 1
	}

	for _, v := range req.Detail {
		r.betMap[p.AccountId()] = append(r.betMap[p.AccountId()], &Bet{
			Bet:   v.Bet,
			Type:  v.Type,
			Round: round, //方便undo
		})
	}

	p.Send(pb.S2U_SPIN_ACK, rsp)
}

// GM: 做牌流程
func (r *Room) GM(p *Player, data []byte) {
	if !implement.IsCheatMode() {
		return
	}

	req := &pb.GMReq{}
	proto.Unmarshal(data, req)

	// 长度不对
	if req.Clear == false && len(req.Type) != config.DiceNum {
		return
	}

	if req.Clear {
		delete(r.gmNumMap, p.AccountId())
	} else {
		r.gmNumMap[p.AccountId()] = req.Type
	}

	rsp := &pb.GMResp{}

	// 发送结果
	p.Send(pb.S2U_GM_ACK, rsp)
}

// GM: 暴击作弊
func (r *Room) GMExtra(p *Player, data []byte) {
	if !implement.IsCheatMode() {
		return
	}
	req := &pb.GMExtraReq{}
	proto.Unmarshal(data, req)

	// 长度不对
	if req.Clear == false && len(req.Extra) == 0 {
		return
	}

	if req.Clear {
		delete(r.gmExtraPay, p.AccountId())
	} else {
		r.gmExtraPay[p.AccountId()] = make([][]pb.AreaType, 0)
		for _, v := range req.Extra {
			r.gmExtraPay[p.AccountId()] = append(r.gmExtraPay[p.AccountId()], v.Type)
		}
	}

	rsp := &pb.GMExtraResp{}

	// 发送结果
	p.Send(pb.S2U_GM_EXTRA_ACK, rsp)
}

// 返回房间信息
func (r *Room) roomInfo(p *Player, data []byte) {
	r.resetRoundIndex()
	rsp := &pb.RoomInfoResp{
		RoundIndex: r.roundIndex,
		RoomState:  int32(r.status),
	}

	for _, p := range r.players {
		wallet := p.user.Profile.Currency

		rsp.Players = append(rsp.Players, &pb.GamePlayerInfo{
			Uid:  p.AccountId(),
			Name: p.Nickname(),
			Wallet: &pb.GameWallet{
				Coin:     p.GetCoin(),
				Currency: wallet.Number,
				Ratio:    wallet.Ratio,
				Rate:     wallet.Rate,
				Symbol:   wallet.Symbol,
			},
		})
	}

	// 给房间数据
	if r.status == RoomStatusIdle {
		sinceTime := time.Since(r.roomStateTime)
		rsp.LeftStateTime = config.GAME_IDLE_TIME - sinceTime.Seconds()
		rsp.TotalStateTime = config.GAME_IDLE_TIME
	} else if r.status == RoomStatusIdle {
		sinceTime := time.Since(r.roomStateTime)
		rsp.LeftStateTime = config.GAME_BET_TIME - sinceTime.Seconds()
		rsp.TotalStateTime = config.GAME_BET_TIME
	}

	p.Send(pb.S2U_ROOM_INFO_ACK, rsp)
}

func (r *Room) onReady(p *Player, data []byte) {
	// req := &pb.GameReadyReq{}
	// proto.Unmarshal(data, req)

	rsp := &pb.GameReadyResp{
		RoundIndex: r.roundIndex,
	}

	if r.status == RoomStatusIdle {
		r.gameReady[p.AccountId()] = true
		rsp.Ret = pb.ErroCode_Success

		if r.isInStatus == false {
			isAllReady := true
			// 判断是否全部准备了？
			for _, player := range r.players {
				if isReady, ok := r.gameReady[player.AccountId()]; !ok || !isReady {
					isAllReady = false
					break
				}
			}
			if isAllReady {
				r.onIdleStatus()
				// r.OnStatus(RoomStatusIdle)
			}
		}

	} else {
		rsp.Ret = pb.ErroCode_NoOpGameState
	}

	p.Send(pb.S2U_GAME_READY_ACK, rsp)
}

func (r *Room) onClear(p *Player, data []byte) {
	// req := &pb.GameClearReq{}
	rsp := &pb.GameClearResp{}
	// proto.Unmarshal(data, req)
	if r.status == RoomStatusBet {
		rsp.Ret = pb.ErroCode_Success
		if _, ok := r.betMap[p.AccountId()]; ok {
			delete(r.betMap, p.AccountId())
		}
	} else {
		rsp.Ret = pb.ErroCode_NoOpGameState
	}
	p.Send(pb.S2U_CLEAR_ACK, rsp)
}

func (r *Room) onUndo(p *Player, data []byte) {
	// req := &pb.GameClearReq{}
	rsp := &pb.GameClearResp{}
	// proto.Unmarshal(data, req)
	if r.status == RoomStatusBet {
		rsp.Ret = pb.ErroCode_Success
		if betMap, ok := r.betMap[p.AccountId()]; ok && len(betMap) > 0 {
			endIndex := len(betMap) - 1
			round := betMap[endIndex].Round

			for i := len(betMap) - 1; i >= 0; i-- {
				if betMap[i].Round == round {
					endIndex = i
				}
			}

			r.betMap[p.AccountId()] = betMap[:endIndex]
		}
	} else {
		rsp.Ret = pb.ErroCode_NoOpGameState
	}
	p.Send(pb.S2U_UNDO_ACK, rsp)
}

func (r *Room) onChangeLevel(p *Player, data []byte) {
	// req := &pb.ChangeRoomReq{}
	// proto.Unmarshal(data, req)

	// 停止游戏流程
	if r.isInStatus {
		r.tw.StopTimer(ROOM_STATE_TW_KEY)
	}
	r.End()
	r.status = RoomStatusIdle
	// 重置历史记录
	r.initHistory()

	rsp := &pb.ChangeRoomResp{}
	p.Send(pb.S2U_CHANGE_ROOM_ACK, rsp)
}

func (r *Room) onHistory(p *Player, data []byte) {
	// req := &pb.HistoryReq{}
	// proto.Unmarshal(data, req)

	rsp := &pb.HistoryResp{DiceNum: config.DiceNum}
	for _, v := range r.history {
		rsp.Draws = append(rsp.Draws, v...)
	}

	p.Send(pb.S2U_HISTORY_ACK, rsp)
}

//------------------------------------------------------------------------------
//	channel
//------------------------------------------------------------------------------

func (r *Room) process() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
			debug.PrintStack()
			go r.process()
		}
	}()

	r.tw.Start()

	for {
		select {
		case p := <-r.join:
			r.Join(p)
		case c := <-r.command:
			r.Command(c)
		case t := <-r.timeout:
			r.TimeOut(t)
		case <-r.close:
			return
		}
	}
}

// Join : 入房流程
func (r *Room) Join(p *Player) {
	if r.IsFull() {
		return
	}

	r.count++
	p.room = r
	r.players[p.AccountId()] = p
	r.Start()

}

// 定时器事件
func (r *Room) TimeOut(t *TimeOut) {
	msgData := t.data
	switch msg := msgData.(type) {
	case RoomStatus:
		r.OnStatus(msg)
	}
}

// Start : 下注開始
func (r *Room) Start() {
	r.resetRoundIndex()
}

// End : 結算流程
func (r *Room) End() {
	r.needNewRoundIndex = true
	r.isInStatus = false

	r.resetRoundIndex()

	for uid, _ := range r.betMap {
		delete(r.betMap, uid)
	}

	// 清空准备状态，以便下一局开始
	for k := range r.gameReady {
		delete(r.gameReady, k)
	}
}

// 时间轮的回调
func (r *Room) twCallBack(data interface{}) {
	r.timeout <- &TimeOut{
		data: data,
	}
}

// 设置定时器方法
func (r *Room) setTimeOut(d time.Duration, key string, data interface{}) {
	if d == 0 {
		r.timeout <- &TimeOut{
			data: data,
		}
		return
	}

	r.tw.AddTimer(key, d, data)
}

// ------------------------------------------------------------------------------
//
//	status
//
// ------------------------------------------------------------------------------
// 切换状态
func (r *Room) OnStatus(status RoomStatus) {
	switch status {
	case RoomStatusIdle:
		//先置回静止状态，方便没有收到可准备状态消息的玩家发送准备
		r.status = RoomStatusIdle
		// 通知客户端可以送准备状态
		r.broadcast(pb.S2U_READY_STATE_PUSH, &pb.RoomCanReadyPush{})
		// r.onIdleStatus()
	case RoomStatusBet:
		r.onBetStatus()
	case RoomStatusSettle:
		r.onSettleStatus()
	}
}

// 静止状态
func (r *Room) onIdleStatus() {
	r.isInStatus = true //是否在游戏状态中。只有玩家全部准备了，才会触发状态
	r.status = RoomStatusIdle
	r.roomStateTime = time.Now() //状态时间

	r.resetRoundIndex()

	sinceTime := time.Since(r.roomStateTime)

	leftStateTime := config.GAME_IDLE_TIME - sinceTime.Seconds()

	r.broadcast(pb.S2U_ROOM_ON_IDLE_PUSH,
		&pb.RoomIdleStatePush{RoomState: (int32)(r.status),
			LeftStateTime:  leftStateTime,
			TotalStateTime: config.GAME_IDLE_TIME,
		})

	// 切换到下一个状态
	r.setTimeOut(config.GAME_IDLE_TIME*time.Second, ROOM_STATE_TW_KEY, RoomStatusBet)
}

// 下注阶段
func (r *Room) onBetStatus() {
	r.status = RoomStatusBet
	r.roomStateTime = time.Now() //状态时间

	sinceTime := time.Since(r.roomStateTime)

	leftStateTime := config.GAME_BET_TIME - sinceTime.Seconds()

	r.broadcast(pb.S2U_ROOM_ON_BET_PUSH,
		&pb.RoomBetStatePush{RoomState: (int32)(r.status),
			LeftStateTime:  leftStateTime,
			TotalStateTime: config.GAME_BET_TIME,
		})

	// 切换到下一个状态
	r.setTimeOut(config.GAME_BET_TIME*time.Second, ROOM_STATE_TW_KEY, RoomStatusSettle)
}

// 结算阶段
func (r *Room) onSettleStatus() {
	r.status = RoomStatusSettle
	r.roomStateTime = time.Now() //状态时间

	//切换到下一个状态 (以)
	r.setTimeOut(config.GAME_OVER_TIME*time.Second, ROOM_STATE_TW_KEY, RoomStatusIdle)

	// 记录暴击数据的
	extraPay := []*pb.ExtraPay{}

	//===============获取第一个玩家，作为作弊数据======
	var p *Player = nil
	for _, _p := range r.players {
		p = _p
		break
	}
	//===============================================
	// 随机单符号暴击区域
	singleExtraPay := r.randSingleExtraPay(p)
	for i := 0; i < len(singleExtraPay); i++ {
		tmp := pb.ExtraPay{}
		tmp.Type = append(tmp.Type, singleExtraPay[i])

		extraPay = append(extraPay, &tmp)
	}

	// // 随机双面暴击区域
	// dobuleExtraPay := r.randDobuleExtraPay(p)
	// for i := 0; i < len(dobuleExtraPay); i++ {
	// 	tmp := pb.ExtraPay{}
	// 	tmp.Type = append(tmp.Type, dobuleExtraPay[i]...)

	// 	extraPay = append(extraPay, &tmp)
	// }

	// 开奖结果
	drawResult := r.randAreaType(p)

	// 统计开奖结果数量
	drawMap := map[pb.AreaType]int32{}
	for _, v := range drawResult {
		if _, ok := drawMap[v]; !ok {
			drawMap[v] = 1
		} else {
			drawMap[v] += 1
		}
	}

	// for uid, betInfos := range r.betMap {
	for uid, p := range r.players {
		betInfos := r.betMap[uid]
		rsp := &pb.SettlePush{Ret: pb.ErroCode_Success, Draw: drawResult, Extra: extraPay, EndCoin: p.user.LoadCoin()}
		// 没有下注信息，只发送暴击的数据，开奖的数据
		if betInfos == nil || len(betInfos) == 0 {
			p.Send(pb.S2U_SETTLE_PUSH, rsp)
			continue
		}

		// 合并多次的下注数据，统一计算
		betInfos = r.mergeBetInfos(betInfos)

		var totalBet float64 = r.getTotalBetMoney(betInfos)

		_, err := r.start(p, totalBet)
		if err != nil {
			rsp.Ret = pb.ErroCode_SysteamError
			// 发送结果
			p.Send(pb.S2U_SETTLE_PUSH, rsp)
			return
		}
		var totalWin float64 = 0
		for _, betInfo := range betInfos {
			result := &pb.Result{Detail: &pb.Bet{
				Bet:  betInfo.Bet,
				Type: betInfo.Type,
			}}
			hasExtraPay := false
			// 单符号下注
			hasExtraPay = r.hasSingleExtraPay(singleExtraPay, betInfo.Type[0])
			winMoney := r.getSingleWinMoney(drawMap, betInfo.Type, betInfo.Bet, hasExtraPay)

			result.HasExtraPay = hasExtraPay
			result.Win = float64(winMoney)

			totalWin += result.Win
			// 保存单注下注结果
			r.signle(p, betInfo.Bet, result.Win, betInfo.Type, hasExtraPay)

			rsp.Result = append(rsp.Result, result)
		}

		// 保存单局开奖结果
		r.round(p, totalBet, totalWin, drawResult)
		if _, err = r.end(p, totalBet, totalWin); err == nil {
			rsp.EndCoin = p.user.LoadCoin()
			p.Send(pb.S2U_SETTLE_PUSH, rsp)
		} else {
			// implement.End失败，返回客户端错误信息
			p.Send(pb.S2U_SETTLE_PUSH, &pb.SettlePush{Ret: pb.ErroCode_SysteamError})
		}
	}

	if len(r.history) > config.MaxHistoryLen {
		r.history = r.history[len(r.history)-config.MaxHistoryLen+1:]
	}

	r.history = append(r.history, drawResult)

	r.End()
}

//------------------------------------------------------------------------------
//	method
//------------------------------------------------------------------------------

func (r *Room) Close() {
	select {
	case r.close <- 1:
	default:
	}
}

// 廣播給房內所有人
func (r *Room) broadcast(ty pb.S2U, data proto.Message) {
	for _, v := range r.players {
		v.Send(ty, data)
	}
}

//------------------------------------------------------------------------------
//	abbrev
//------------------------------------------------------------------------------

func (r *Room) IsFull() bool {
	return r.count >= 1
}

// 随机出局号
func (r *Room) resetRoundIndex() {
	if r.needNewRoundIndex {
		r.roundIndex = implement.CreateRoundIndex()
		r.needNewRoundIndex = false
	}
}

// 获取数据库的操作对像（需要甲方实现）
func (r *Room) getDB() (db *gorm.DB, err error) {
	return nil, errors.New("no implement")
}

// ------------------------------------------------------------------------------
// logic
// ------------------------------------------------------------------------------
// 随机路单数据
func (r *Room) initHistory() {
	if len(r.history) != config.MaxHistoryLen {
		r.history = make([][]pb.AreaType, config.MaxHistoryLen)
	}

	for i := 0; i < config.MaxHistoryLen; i++ {
		r.history[i] = r.randAreaType(nil)
	}
}

// 随机开奖结果
func (r *Room) randAreaType(p *Player) []pb.AreaType {
	ret := []pb.AreaType{}

	// 作弊的数据返回
	if p != nil && implement.IsCheatMode() {
		if gmNumList, ok := r.gmNumMap[p.AccountId()]; ok {
			if len(gmNumList) > 0 {
				ret = append(ret, gmNumList...)
				return ret
			}
		}
	}

	// 正常数据返回
	for i := 0; i < config.DiceNum; i++ {
		ret = append(ret, (pb.AreaType)(rand.Intn(config.DiceFaceNum)))
	}

	return ret
}

// 随机单符号的暴击区域
func (r *Room) randSingleExtraPay(p *Player) []pb.AreaType {
	ret := []pb.AreaType{}

	if config.SingleExtraPayRate == 0 {
		return ret
	}

	// 作弊的数据返回
	if p != nil && implement.IsCheatMode() {
		if gmExtraPay, ok := r.gmExtraPay[p.AccountId()]; ok {
			for _, v := range gmExtraPay {
				if len(v) == 1 {
					ret = append(ret, v[0])
				}
			}
			return ret
		}
	}

	// 正常数据返回
	for i := 0; i < config.DiceFaceNum; i++ {
		if rand.Intn(100) < config.SingleExtraPayRate {
			ret = append(ret, (pb.AreaType)(i))
		}
	}

	return ret
}

// // 随机双面的暴击区域
// func (r *Room) randDobuleExtraPay(p *Player) [][]pb.AreaType {
// 	ret := [][]pb.AreaType{}

// 	if config.DoubleExtraPayRate == 0 {
// 		return ret
// 	}

// 	// 作弊的数据返回
// 	if p != nil && implement.IsCheatMode() {
// 		if gmExtraPay, ok := r.gmExtraPay[p.AccountId()]; ok {
// 			for _, v := range gmExtraPay {
// 				if len(v) == 2 {
// 					ret = append(ret, v)
// 				}
// 			}
// 			return ret
// 		}
// 	}

// 	// 正常数据返回
// 	for i := 0; i < len(doubleBetArea); i++ {
// 		if rand.Intn(100) < config.DoubleExtraPayRate {
// 			ret = append(ret, doubleBetArea[i])
// 		}
// 	}

// 	return ret
// }

// 是否中奖
func (r *Room) hasReward(drawMap map[pb.AreaType]int32, areaType []pb.AreaType) bool {
	for i := 0; i < len(areaType); i++ {
		if num, ok := drawMap[areaType[i]]; !ok || num <= 0 {
			return false
		}
	}
	return true
}

// 获取相同符号的单符号
func (r *Room) getSingleSameNum(drawMap map[pb.AreaType]int32, areaType pb.AreaType) int32 {
	if _, ok := drawMap[areaType]; !ok {
		return 0
	} else {
		return drawMap[areaType]
	}
}

// 是否有单符号暴击
func (r *Room) hasSingleExtraPay(singleExtraPay []pb.AreaType, areaType pb.AreaType) bool {
	for _, v := range singleExtraPay {
		if v == areaType {
			return true
		}
	}
	return false
}

// 获取赔率
func (r *Room) getSingleRate(drawMap map[pb.AreaType]int32, areaType pb.AreaType) float64 {
	num := r.getSingleSameNum(drawMap, areaType)

	if _, ok := config.SingleRate[num]; !ok {
		return 0
	} else {
		return config.SingleRate[num]
	}
}

// 获取暴击倍数
func (r *Room) getSingleExtraPayMultiple(drawMap map[pb.AreaType]int32, areaType pb.AreaType) float64 {
	num := r.getSingleSameNum(drawMap, areaType)

	if _, ok := config.SingleExtraPayMultiple[num]; !ok {
		return 0
	} else {
		return config.SingleExtraPayMultiple[num]
	}
}

// 计算单符号赢钱, isExtraPay 是否有暴击
func (r *Room) getSingleWinMoney(drawMap map[pb.AreaType]int32, areaType []pb.AreaType, betMoney float64, isExtraPay bool) float64 {
	if !r.hasReward(drawMap, areaType) {
		return 0
	}
	// 先获取赔率
	rate := r.getSingleRate(drawMap, areaType[0])

	if isExtraPay {
		rate = r.getSingleExtraPayMultiple(drawMap, areaType[0])
	}
	return betMoney * rate
}

// 是否有双符号暴击
func (r *Room) hasDoubleExtraPay(dobuleExtraPay [][]pb.AreaType, areaType []pb.AreaType) bool {
	for _, v := range dobuleExtraPay {
		if (v[0] == areaType[0] && v[1] == areaType[1]) ||
			(v[1] == areaType[0] && v[0] == areaType[1]) {
			return true
		}
	}
	return false
}

// // 计算单符号赢钱, isExtraPay 是否有暴击
// func (r *Room) getDoubleWinMoney(drawMap map[pb.AreaType]int32, areaType []pb.AreaType, betMoney float64, isExtraPay bool) float64 {

// 	if !r.hasReward(drawMap, areaType) {
// 		return 0
// 	}

// 	// 先获取赔率
// 	rate := config.DoubleRate

// 	if isExtraPay {
// 		rate *= config.DoubleExtraPayMultiple
// 	}
// 	return betMoney * rate
// }

func (r *Room) getTotalBetMoney(betInfo BetInfo) float64 {
	var ret float64 = 0
	for _, v := range betInfo {
		ret += v.Bet
	}
	return ret
}

func (r *Room) isSameAreaTypeList(aList []pb.AreaType, bList []pb.AreaType) bool {
	if len(aList) != len(bList) {
		return false
	}

	if len(aList) == 1 {
		return aList[0] == bList[0]
	}

	aNums := []int{}
	for i := 0; i < len(aList); i++ {
		aNums = append(aNums, int(aList[i]))
	}

	sort.Ints(aNums)

	bNums := []int{}
	for i := 0; i < len(bList); i++ {
		bNums = append(bNums, int(bList[i]))
	}

	sort.Ints(bNums)

	for i := 0; i < len(aNums); i++ {
		if aNums[i] != bNums[i] {
			return false
		}
	}
	return true
}

func (r *Room) mergeBetInfos(betInfo BetInfo) BetInfo {
	ret := make(BetInfo, 0)

	for i := 0; i < len(betInfo); i++ {
		tmp := betInfo[i]
		isFound := false
		for j := 0; j < len(ret); j++ {
			if r.isSameAreaTypeList(ret[j].Type, tmp.Type) {
				ret[j].Bet += tmp.Bet
				isFound = true
				break
			}
		}

		if isFound == false {
			ret = append(ret, &Bet{
				Type: tmp.Type,
				Bet:  tmp.Bet,
			})
		}
	}
	return ret
}

//------------------------------------------------------------------------------
//	save db
//------------------------------------------------------------------------------

func (r *Room) start(p *Player, betMoney float64) (*types.UserValue, error) {
	u := &types.UserValue{
		RoundIndex: r.roundIndex, //填入局號 (CreateRoundIndex)

		LogIndex:  implement.CreateLogIndex(), //填入局號 (CreateWagersIndex)
		AccountID: int32(p.AccountId()),
		APIID:     int32(p.ApiId()),
		Bet:       betMoney,
		Win:       0,
		Balance:   betMoney,

		CreateTime:     time.Now(),                         //該單號產生時間
		CurrencyNumber: int32(p.GetCurrency()),             //玩家資料(PlayerInfo)
		Ratio:          p.GetRatio(),                       //玩家資料(PlayerInfo)
		Rate:           p.GetRate(),                        //玩家資料(PlayerInfo)
		SubAgentCode:   int32(p.user.Profile.SubAgentCode), //玩家資料(PlayerInfo)
		Verify:         true,                               //固定為true
	}

	err := implement.Start(p.user, u)
	u.Verify = (err == nil)

	return u, err
}

func (r *Room) end(p *Player, totalBet float64, totalWin float64) (float64, error) {
	u := &types.UserValue{
		RoundIndex: r.roundIndex, //填入局號 (CreateRoundIndex)

		LogIndex:  implement.CreateLogIndex(), //填入局號 (CreateWagersIndex)
		AccountID: int32(p.AccountId()),
		APIID:     int32(p.ApiId()),
		Bet:       0,
		Win:       totalWin,
		Balance:   totalWin - totalBet,

		CreateTime:     time.Now(),                         //該單號產生時間
		CurrencyNumber: int32(p.GetCurrency()),             //玩家資料(PlayerInfo)
		Ratio:          p.GetRatio(),                       //玩家資料(PlayerInfo)
		Rate:           p.GetRate(),                        //玩家資料(PlayerInfo)
		SubAgentCode:   int32(p.user.Profile.SubAgentCode), //玩家資料(PlayerInfo)
		Verify:         true,                               //固定為true
	}

	newMoney, err := implement.End(p.user, u)
	u.Verify = (err == nil)
	// 将新的钱写的钱包
	return newMoney, err
}

func (r *Room) signle(p *Player, bet float64, win float64, target []pb.AreaType, hasExtraPay bool) error {
	targetIndex := 0
	if len(target) == 1 {
		targetIndex = singleRoundTargetIndex[int32(target[0])]
	} else if len(target) == 2 {
		nums := []int{
			int(target[0]),
			int(target[1]),
		}

		sort.Ints(nums)
		targetIndex = (nums[0]+1)*10 + nums[1]
	} else {
		return errors.New("target len error")
	}

	d := &types.SingleRound{
		RoundIndex: r.roundIndex, //填入局號 (CreateRoundIndex)
		AccountID:  int32(p.AccountId()),

		ExtraPay:   hasExtraPay,
		Target:     int32(targetIndex),
		Bet:        bet,
		Win:        win,
		Odds:       win / bet,
		CreateTime: time.Now(), //該單號產生時間
	}

	db, err := r.getDB()
	if err == nil {
		return db.Create(d).Error
	}

	return err
}

func (r *Room) round(p *Player, totalBet float64, totalWin float64, drawResult []pb.AreaType) error {
	// 将数组转换为字符串切片
	strSlice := make([]string, len(drawResult))
	for i, num := range drawResult {
		strSlice[i] = fmt.Sprintf("%d", int32(num))
	}

	d := &types.RoundValue{
		AlterID:    0, //道具id
		ItemReward: 0, // 道具奖励金额

		RoundIndex: r.roundIndex, //填入局號 (CreateRoundIndex)
		AccountID:  int32(p.AccountId()),
		APIID:      int32(p.ApiId()),

		Bet: totalBet,
		Win: totalWin,

		DrawResult: strings.Join(strSlice, ","),
		PreMoney:   p.GetCoin(),
		PostMoney:  p.GetCoin() + totalWin - totalBet,

		CreateTime:     time.Now(),
		CurrencyNumber: int32(p.GetCurrency()),             //玩家資料(PlayerInfo)
		Ratio:          p.GetRatio(),                       //玩家資料(PlayerInfo)
		Rate:           p.GetRate(),                        //玩家資料(PlayerInfo)
		SubAgentCode:   int32(p.user.Profile.SubAgentCode), //玩家資料(PlayerInfo)
		Verify:         true,                               //固定為true
	}

	db, err := r.getDB()
	if err == nil {
		return db.Create(d).Error
	}

	return err
}
