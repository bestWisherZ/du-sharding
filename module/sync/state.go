package sync

type state struct {
	//max height of block has been synced
	blocksHasSynced uint64
	//num of block in cache waiting to be committed
	blocksInCache int
}

type getStateFn func() state
