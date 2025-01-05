package ws

import (
	"context"

	"github.com/BBleae/solana-go"
	"github.com/BBleae/solana-go/rpc"
)

type TransactionResult struct {
	Signature   string `json:"signature"`
	Slot        uint64 `json:"slot"`
	Transaction struct {
		Transaction *rpc.TransactionResultEnvelope `json:"transaction" bin:"optional"`
		Meta        *rpc.TransactionMeta           `json:"meta,omitempty" bin:"optional"`
		Version     rpc.TransactionVersion         `json:"version"`
	} `json:"transaction"`
}

type TransactionResult_Triton struct {
	Value TransactionResult `json:"value"`
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

type TransactionAccountsFilter_Triton struct {
	Include  []string `json:"include,omitempty"`
	Exclude  []string `json:"exclude,omitempty"`
	Required []string `json:"required,omitempty"`
}

type TransactionSubscribeFilter_Triton struct {
	Vote      bool                             `json:"vote,omitempty"`
	Failed    bool                             `json:"failed,omitempty"`
	Signature string                           `json:"signature,omitempty"`
	Accounts  TransactionAccountsFilter_Triton `json:"accounts,omitempty"`
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
	isTriton ...bool,
) (*TransactionSubscription, error) {
	params := []interface{}{}

	// Check if isTriton is provided and true
	if len(isTriton) > 0 && isTriton[0] {
		tritonFilter := TransactionSubscribeFilter_Triton{
			Vote:      filter.Vote,
			Failed:    filter.Failed,
			Signature: filter.Signature,
			Accounts: TransactionAccountsFilter_Triton{
				Include:  filter.AccountInclude,
				Exclude:  filter.AccountExclude,
				Required: filter.AccountRequired,
			},
		}
		params = append(params, tritonFilter)
	} else {
		params = append(params, filter)
	}

	if options != nil {
		params = append(params, options)
	}

	genSub, err := cl.subscribe(
		params,
		nil,
		"transactionSubscribe",
		"transactionUnsubscribe",
		func(msg []byte) (interface{}, error) {
			if len(isTriton) > 0 && isTriton[0] {
				var res TransactionResult_Triton
				err := decodeResponseFromMessage(msg, &res)
				if res.Value.Signature == "" {
					tx, err := res.Value.Transaction.Transaction.GetTransaction()
					if err != nil {
						return nil, err
					}
					res.Value.Signature = tx.Signatures[0].String()
				}
				return &res.Value, err
			} else {
				var res TransactionResult
				err := decodeResponseFromMessage(msg, &res)
				return &res, err
			}
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
