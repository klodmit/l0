package storage

import (
	"context"
	"errors"
	"l0/internal/model"
	"log"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewOrderRepo(pool *pgxpool.Pool, log *slog.Logger) *OrderRepository {
	return &OrderRepository{pool: pool, log: log}
}

const (
	qInsertOrder = `
INSERT INTO orders (
  order_uid, track_number, entry, locale, internal_signature, customer_id,
  delivery_service, shardkey, sm_id, date_created, oof_shard)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	qInsertDelivery = `
INSERT INTO deliveries (
  order_uid, name, phone, zip, city, address, region, email)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8);`
	qInsertPayment = `
INSERT INTO payments (
  order_uid, transaction, request_id, currency, provider, amount, payment_dt,
  bank, delivery_cost, goods_total, custom_fee)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11);`
	qInsertItem = `
INSERT INTO items (
  order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12);`
)

func (r *OrderRepository) SaveOrder(ctx context.Context, o model.Order) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func(tx pgx.Tx, ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil {
			log.Fatalf("Rollback failed:%s", err)
		}
	}(tx, ctx)

	if _, err = tx.Exec(ctx, qInsertOrder, o.OrderUID, o.TrackNumber, o.Entry, o.Locale, o.InternalSignature, o.CustomerID,
		o.DeliveryService, o.Shardkey, o.SmID, o.DateCreated, o.OofShard,
	); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, qInsertDelivery,
		o.OrderUID,
		o.Delivery.Name, o.Delivery.Phone, o.Delivery.Zip, o.Delivery.City,
		o.Delivery.Address, o.Delivery.Region, o.Delivery.Email,
	); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, qInsertPayment,
		o.OrderUID,
		o.Payment.Transaction, o.Payment.RequestID, o.Payment.Currency, o.Payment.Provider,
		o.Payment.Amount, o.Payment.PaymentDt, o.Payment.Bank, o.Payment.DeliveryCost,
		o.Payment.GoodsTotal, o.Payment.CustomFee,
	); err != nil {
		return err
	}
	if len(o.Items) > 0 {
		batch := &pgx.Batch{}
		for _, it := range o.Items {
			batch.Queue(qInsertItem,
				o.OrderUID, it.ChrtID, it.TrackNumber, it.Price, it.Rid, it.Name,
				it.Sale, it.Size, it.TotalPrice, it.NmID, it.Brand, it.Status,
			)
		}
		br := tx.SendBatch(ctx, batch)
		for range o.Items {
			if _, err = br.Exec(); err != nil {
				_ = br.Close()
				return err
			}
		}
		if err = br.Close(); err != nil {
			return err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	r.log.Info("order saved", "order_uid", o.OrderUID, "items", len(o.Items))
	return nil
}

var ErrNotFound = errors.New("order not found")

const qSelectOrder = `
SELECT
  o.order_uid, o.track_number, o.entry, o.locale, o.internal_signature, o.customer_id,
  o.delivery_service, o.shardkey, o.sm_id, o.date_created, o.oof_shard,

  d.name, d.phone, d.zip, d.city, d.address, d.region, d.email,

  p.transaction, p.request_id, p.currency, p.provider,
  (p.amount)::float8, p.payment_dt, p.bank,
  (p.delivery_cost)::float8, (p.goods_total)::float8, (p.custom_fee)::float8
FROM orders o
JOIN deliveries d ON d.order_uid = o.order_uid
JOIN payments   p ON p.order_uid = o.order_uid
WHERE o.order_uid = $1;`

const qSelectItems = `
SELECT chrt_id, track_number, (price)::float8, rid, name, (sale)::float8, size,
       (total_price)::float8, nm_id, brand, status
FROM items
WHERE order_uid = $1
ORDER BY item_id;`

func (r *OrderRepository) GetOrder(ctx context.Context, orderUID string) (model.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var o model.Order

	err := r.pool.QueryRow(ctx, qSelectOrder, orderUID).Scan(
		&o.OrderUID, &o.TrackNumber, &o.Entry, &o.Locale, &o.InternalSignature, &o.CustomerID,
		&o.DeliveryService, &o.Shardkey, &o.SmID, &o.DateCreated, &o.OofShard,

		&o.Delivery.Name, &o.Delivery.Phone, &o.Delivery.Zip, &o.Delivery.City,
		&o.Delivery.Address, &o.Delivery.Region, &o.Delivery.Email,

		&o.Payment.Transaction, &o.Payment.RequestID, &o.Payment.Currency, &o.Payment.Provider,
		&o.Payment.Amount, &o.Payment.PaymentDt, &o.Payment.Bank,
		&o.Payment.DeliveryCost, &o.Payment.GoodsTotal, &o.Payment.CustomFee,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Order{}, ErrNotFound
		}
		return model.Order{}, err
	}

	rows, err := r.pool.Query(ctx, qSelectItems, orderUID)
	if err != nil {
		return model.Order{}, err
	}
	defer rows.Close()

	items := make([]model.Item, 0, 8)
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(
			&it.ChrtID, &it.TrackNumber, &it.Price, &it.Rid, &it.Name, &it.Sale,
			&it.Size, &it.TotalPrice, &it.NmID, &it.Brand, &it.Status,
		); err != nil {
			return model.Order{}, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return model.Order{}, err
	}
	o.Items = items

	return o, nil
}
