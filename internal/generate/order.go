package generate

import (
	"math/rand"
	"time"

	"l0/internal/model"
)

var brands = []string{"Vivienne Sabo", "L'Oreal", "Maybelline", "NYX"}
var cities = []string{"Moscow", "Kazan", "Kiryat Mozkin", "Saint Petersburg"}

func randStr(prefix string, n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return prefix + string(b)
}

func ValidOrder() model.Order {
	now := time.Now().UTC()
	uid := randStr("ord_", 12)
	track := randStr("TRK", 10)

	itemCount := 1 + rand.Intn(3)
	items := make([]model.Item, 0, itemCount)
	var goodsTotal float64

	for i := 0; i < itemCount; i++ {
		price := float64(100 + rand.Intn(900)) // 100..999
		sale := float64([]int{0, 10, 20, 30}[rand.Intn(4)])
		total := price - (price*sale)/100
		goodsTotal += total

		items = append(items, model.Item{
			ChrtID:      int64(9000000 + rand.Intn(999999)),
			TrackNumber: track,
			Price:       price,
			Rid:         randStr("RID_", 12),
			Name:        "Item " + randStr("", 5),
			Sale:        sale,
			Size:        "M",
			TotalPrice:  total,
			NmID:        int64(2000000 + rand.Intn(999999)),
			Brand:       brands[rand.Intn(len(brands))],
			Status:      202,
		})
	}

	deliv := model.Delivery{
		Name:    "Test User",
		Phone:   "+1234567890",
		Zip:     "123456",
		City:    cities[rand.Intn(len(cities))],
		Address: "Main st. 1",
		Region:  "RegionX",
		Email:   "test@example.com",
	}

	pay := model.Payment{
		Transaction:  uid,
		RequestID:    "",
		Currency:     "USD",
		Provider:     "wbpay",
		Amount:       goodsTotal + 1500, // просто для теста
		PaymentDt:    now.Unix(),
		Bank:         "alpha",
		DeliveryCost: 1500,
		GoodsTotal:   goodsTotal,
		CustomFee:    0,
	}

	return model.Order{
		OrderUID:          uid,
		TrackNumber:       track,
		Entry:             "WBIL",
		Delivery:          deliv,
		Payment:           pay,
		Items:             items,
		Locale:            "en",
		InternalSignature: "",
		CustomerID:        "test_customer",
		DeliveryService:   "meest",
		Shardkey:          "9",
		SmID:              99,
		DateCreated:       now,
		OofShard:          "1",
	}
}

// InvalidPayload возвращает JSON-строку, синтаксически валидную,
// но заведомо несовместимую с model.Order (сломанные типы/поля).
func InvalidPayload() []byte {
	// amount как строка, payment_dt строка,
	// items — строка вместо массива и т.д.
	return []byte(`{
	  "order_uid": 123,
	  "track_number": 42,
	  "payment": {
	    "amount": "not-a-number",
	    "payment_dt": "nope",
	    "currency": 100500
	  },
	  "items": "should-be-array-not-string",
	  "date_created": "invalid-time-format"
	}`)
}
