package ws

import (
	"context"

	"github.com/BBleae/solana-go"
	"github.com/BBleae/solana-go/rpc"
)

type TransactionResult struct {
	Signature   string                         `json:"signature"`
	Slot        uint64                         `json:"slot"`
	Transaction rpc.GetParsedTransactionResult `json:"transaction"`
}

type TransactionSubscribeFilter struct {
	// TODO: bool 也 omitempty 不知道是否合理。
	Vote            bool     `json:"vote,omitempty"`
	Failed          bool     `json:"failed,omitempty"`
	Signature       string   `json:"signature,omitempty"`
	AccountInclude  []string `json:"accountInclude,omitempty"`
	AccountExclude  []string `json:"accountExclude,omitempty"`
	AccountRequired []string `json:"accountRequired,omitempty"`
}

type TransactionSubscribeOptions struct {
	Commitment         rpc.CommitmentType  `json:"commitment,omitempty"`
	Encoding           solana.EncodingType `json:"encoding,omitempty"`
	TransactionDetails string              `json:"transaction_details,omitempty"`
	ShowRewards        bool                `json:"showRewards,omitempty"`
	// 这个 0 的值必须传递过去，不能 omitempty。
	MaxSupportedTransactionVersion int `json:"maxSupportedTransactionVersion"`
}

func (cl *Client) TransactionSubscribe(
	filter TransactionSubscribeFilter,
	options *TransactionSubscribeOptions,
) (*TransactionSubscription, error) {

	params := []interface{}{filter}
	if options != nil {
		params = append(params, options)
	}

	genSub, err := cl.subscribe(
		params,
		nil,
		"transactionSubscribe",
		"transactionUnsubscribe",
		func(msg []byte) (interface{}, error) {
			var res TransactionResult
			err := decodeResponseFromMessage(msg, &res)
			return &res, err
		},
	)
	if err != nil {
		return nil, err
	}
	return &TransactionSubscription{
		sub: genSub,
	}, nil
}

type TransactionSubscription struct {
	sub *Subscription
}

func (sw *TransactionSubscription) Recv(ctx context.Context) (*TransactionResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case d, ok := <-sw.sub.stream:
		if !ok {
			return nil, ErrSubscriptionClosed
		}
		return d.(*TransactionResult), nil
	case err := <-sw.sub.err:
		return nil, err
	}
}

func (sw *TransactionSubscription) Err() <-chan error {
	return sw.sub.err
}

func (sw *TransactionSubscription) Response() <-chan *TransactionResult {
	typedChan := make(chan *TransactionResult, 1)
	go func(ch chan *TransactionResult) {
		// TODO: will this subscription yield more than one result?
		d, ok := <-sw.sub.stream
		if !ok {
			return
		}
		ch <- d.(*TransactionResult)
	}(typedChan)
	return typedChan
}

func (sw *TransactionSubscription) Unsubscribe() {
	sw.sub.Unsubscribe()
}
