package bitcoin

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
)

type Generator interface {
	Header(extranonce, nonce string) (string, error) // On aux generation, on work verfication, and possibily even work submission
	Sum() (*big.Int, error)                          // On work verification, many, more than than header generation
	Submit() (string, error)                         // On submission
}

var jobCounter int

func GenerateWork(template *Template, chainName, arbitrary, poolPayoutPubScriptKey string, reservedArbitraryByteLength int) (*BitcoinBlock, Work, error) { // On trigger
	if template == nil {
		return nil, nil, errors.New("Template cannot be null")
	}

	var err error
	block := BitcoinBlock{}

	block.init(GetChain(chainName))
	block.Template = template

	block.reversePrevBlockHash, err = reverseHex4Bytes(block.Template.PrevBlockHash)
	if err != nil {
		m := "Invalid previous block hash hex: " + err.Error()
		return nil, nil, errors.New(m)
	}

	arbitraryBytes := bytesWithLengthHeader([]byte(arbitrary))
	arbitraryByteLength := uint(len(arbitraryBytes) + reservedArbitraryByteLength)
	arbitraryHex := hex.EncodeToString(arbitraryBytes)

	block.coinbaseInitial = block.Template.CoinbaseInitial(arbitraryByteLength).Serialize()
	block.coinbaseFinal = arbitraryHex + block.Template.CoinbaseFinal(poolPayoutPubScriptKey).Serialize()
	block.merkleSteps, err = block.Template.MerkleSteps()
	if err != nil {
		return nil, nil, err
	}

	work := make(Work, 8)
	work[0] = fmt.Sprintf("%08x", jobCounter) // Job ID
	work[1] = block.reversePrevBlockHash
	work[2] = block.coinbaseInitial
	work[3] = block.arbitrary + block.coinbaseFinal
	work[4] = block.merkleSteps
	work[5] = fmt.Sprintf("%08x", block.Template.Version)
	work[6] = block.Template.Bits
	work[7] = fmt.Sprintf("%x", block.Template.CurrentTime)

	jobCounter++

	return &block, work, nil

}

func (b *BitcoinBlock) Header(extranonce, nonce string) (string, error) {
	if b.Template == nil {
		return "", errors.New("Generate work first")
	}

	var err error
	coinbase := Coinbase{
		CoinbaseInital: b.coinbaseInitial,
		Arbitrary:      extranonce,
		CoinbaseFinal:  b.coinbaseFinal,
	}

	b.coinbase = coinbase.Serialize()
	coinbaseHashed, err := b.chain.CoinbaseDigest(b.coinbase)
	if err != nil {
		return "", err
	}

	merkleRoot, err := makeHeaderMerkleRoot(coinbaseHashed, b.merkleSteps)
	if err != nil {
		return "", err
	}

	t := b.Template
	b.header, err = blockHeader(uint(t.Version), t.PrevBlockHash, merkleRoot, fmt.Sprintf("%x", t.CurrentTime), t.Bits, nonce)
	if err != nil {
		return "", err
	}

	return b.header, nil
}

func (b *BitcoinBlock) Sum() (*big.Int, error) {
	if b.chain == nil {
		return nil, errors.New("CalculateSum: Missing blockchain interface")
	}
	if b.header == "" {
		return nil, errors.New("Generate header first")
	}

	digest, err := b.chain.HeaderDigest(b.header)
	if err != nil {
		return nil, err
	}

	digestBytes, err := hex.DecodeString(digest)
	digestBytes = reverse(digestBytes)

	return new(big.Int).SetBytes(digestBytes), nil
}

func (b *BitcoinBlock) Submit() (string, error) {
	if b.header == "" {
		return "", errors.New("Generate header first")
	}

	transactionPool := make([]string, len(b.Template.Transactions))
	for i, transaction := range b.Template.Transactions {
		transactionPool[i] = transaction.Data
	}

	return b.createSubmissionHex(), nil
}