package indexer

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/taikoxyz/taiko-mono/packages/eventindexer"
	"github.com/taikoxyz/taiko-mono/packages/eventindexer/contracts/swap"
)

var (
	minLiquidityAddedAmount = big.NewInt(10000000000000000)
)

func (svc *Service) saveLiquidityAddedEvents(
	ctx context.Context,
	chainID *big.Int,
	events *swap.SwapMintIterator,
) error {
	if !events.Next() || events.Event == nil {
		log.Infof("no LiquidityAdded events")
		return nil
	}

	for {
		event := events.Event

		if err := svc.saveLiquidityAddedEvent(ctx, chainID, event); err != nil {
			eventindexer.LiquidityAddedEventsProcessedError.Inc()

			return errors.Wrap(err, "svc.saveSwapEvent")
		}

		if !events.Next() {
			return nil
		}
	}
}

func (svc *Service) saveLiquidityAddedEvent(
	ctx context.Context,
	chainID *big.Int,
	event *swap.SwapMint,
) error {
	tx, _, err := svc.ethClient.TransactionByHash(ctx, event.Raw.TxHash)
	if err != nil {
		return err
	}

	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return err
	}

	log.Infof("liquidityAdded event for sender %v, amount0: %v, amount1: %v",
		from.Hex(),
		event.Amount0.String(),
		event.Amount1.String(),
	)

	// we only want events with > 0.1 ETH swap
	if event.Amount0.Cmp(minLiquidityAddedAmount) <= 0 && event.Amount1.Cmp(minLiquidityAddedAmount) <= 0 {
		log.Infof("skipping liquidityAdded event, min trade too low. amountIn: %v, amountOut: %v",
			event.Amount0.String(),
			event.Amount1.String(),
		)

		return nil
	}

	marshaled, err := json.Marshal(event)
	if err != nil {
		return errors.Wrap(err, "json.Marshal(event)")
	}

	_, err = svc.eventRepo.Save(ctx, eventindexer.SaveEventOpts{
		Name:    eventindexer.EventNameMint,
		Data:    string(marshaled),
		ChainID: chainID,
		Event:   eventindexer.EventNameMint,
		Address: from.Hex(),
	})
	if err != nil {
		return errors.Wrap(err, "svc.eventRepo.Save")
	}

	eventindexer.LiquidityAddedEventsProcessed.Inc()

	return nil
}