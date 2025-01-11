package ws

import (
	"context"

	"github.com/Sig9h/solana-go"
	"github.com/Sig9h/solana-go/rpc"
)

type TransactionEvent struct {
	IsJSONParsed bool                     `json:"isJSONParsed"`
	Signature    string                   `json:"signature"`
	Result       *TransactionResult       `json:"result,omitempty"`
	ParsedResult *ParsedTransactionResult `json:"parsedResult"`
}

type TransactionResult struct {
	Signature   string `json:"signature"`
	Slot        uint64 `json:"slot"`
	Transaction struct {
		Transaction *rpc.TransactionResultEnvelope `json:"transaction" bin:"optional"`
		Meta        *rpc.TransactionMeta           `json:"meta,omitempty" bin:"optional"`
		Version     rpc.TransactionVersion         `json:"version"`
	} `json:"transaction"`
}
type ParsedTransactionResult struct {
	Signature   string                         `json:"signature"`
	Slot        uint64                         `json:"slot"`
	Transaction rpc.GetParsedTransactionResult `json:"transaction"`
}

type TritonTXResult[T TransactionResult | ParsedTransactionResult] struct {
	Value T `json:"value"`
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
			isJSONParsed := options != nil && options.Encoding == solana.EncodingJSONParsed
			event := &TransactionEvent{
				IsJSONParsed: isJSONParsed,
			}

			if len(isTriton) > 0 && isTriton[0] {
				if isJSONParsed {
					var res TritonTXResult[ParsedTransactionResult]
					err := decodeResponseFromMessage(msg, &res)
					if err != nil {
						return nil, err
					}
					// TODO: Sig 修补？
					event.ParsedResult = &res.Value
					event.Signature = res.Value.Signature
				} else {
					var res TritonTXResult[TransactionResult]
					err := decodeResponseFromMessage(msg, &res)
					if err != nil {
						return nil, err
					}
					// TODO: 需要确认下这个修补的必要性。
					if res.Value.Signature == "" {
						tx, err := res.Value.Transaction.Transaction.GetTransaction()
						if err != nil {
							return nil, err
						}
						res.Value.Signature = tx.Signatures[0].String()
					}
					event.Result = &res.Value
					event.Signature = res.Value.Signature
				}
			} else {
				if isJSONParsed {
					err := decodeResponseFromMessage(msg, &event.ParsedResult)
					if err != nil {
						return nil, err
					}
					event.Signature = event.ParsedResult.Signature
				} else {
					err := decodeResponseFromMessage(msg, &event.Result)
					if err != nil {
						return nil, err
					}
					event.Signature = event.Result.Signature
				}
			}

			return event, nil
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

func (sw *TransactionSubscription) Recv(ctx context.Context) (*TransactionEvent, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case d, ok := <-sw.sub.stream:
		if !ok {
			return nil, ErrSubscriptionClosed
		}
		return d.(*TransactionEvent), nil
	case err := <-sw.sub.err:
		return nil, err
	}
}

func (sw *TransactionSubscription) Err() <-chan error {
	return sw.sub.err
}

func (sw *TransactionSubscription) Response() <-chan *TransactionEvent {
	typedChan := make(chan *TransactionEvent, 1)
	go func(ch chan *TransactionEvent) {
		// TODO: will this subscription yield more than one result?
		d, ok := <-sw.sub.stream
		if !ok {
			return
		}
		ch <- d.(*TransactionEvent)
	}(typedChan)
	return typedChan
}

func (sw *TransactionSubscription) Unsubscribe() {
	sw.sub.Unsubscribe()
}
