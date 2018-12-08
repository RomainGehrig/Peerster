package blockchain

type BlockchainHandler struct {
}

func NewBlockchainHandler() *BlockchainHandler {
	return &BlockchainHandler{}
}

func (b *BlockchainHandler) RunBlockchainHandler() {

}

func (b *BlockchainHandler) HandleTxPublish(tx *TxPublish) {
	// Should not block
	fmt.Println("Got txpublish:", tx)
}

func (b *BlockchainHandler) HandleBlockPublish(blk *BlockPublish) {
	// Should not block
	fmt.Println("Got blockpublish:", blk)
}
