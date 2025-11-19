package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var masterDB *pgxpool.Pool

func main() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var err error
	masterDB, err = pgxpool.New(ctx, "postgres://temporal:temporal@localhost:5432/tsdb")
	if err != nil {
		log.Println("pgxpool.New (master):", err)
		return
	}
	err = masterDB.Ping(ctx)
	if err != nil {
		log.Println("masterDB.Ping:", err)
		return
	}
	defer masterDB.Close()

	// Вставка тестовых данных
	// Раскомментируйте для запуска:

	// Вставка 1 миллион клиентов
	// if err := BatchInsertCustomers(1000000); err != nil {
	// 	log.Fatal("BatchInsertCustomers:", err)
	// }

	// Вставка 2 миллиона счетов для клиентов
	// if err := BatchInsertAccounts(2000000, 1000000); err != nil {
	// 	log.Fatal("BatchInsertAccounts:", err)
	// }

	// Вставка 10 миллионов платежных ордеров
	//if err := BatchInsertPaymentOrders(10000000, 2000000); err != nil {
	//	log.Fatal("BatchInsertPaymentOrders:", err)
	//}

	// Выполнение параллельных транзакций
	go run1(ctx, 48, 47, 20.00, "RUB")
	go run1(ctx, 47, 50, 100.00, "RUB")

	//RunParallelTransactions(10, 50, 5000)
	time.Sleep(5 * time.Second)
	log.Println("Готово!")
}

// BatchUserLog выполняет массовую вставку логов пользователей в таблицу user_audit_log
func BatchUserLog(data [][]any) {
	cnt, err := masterDB.CopyFrom(context.Background(), pgx.Identifier{"user_audit_log"}, []string{"action", "user_email", "details", "created_at"}, pgx.CopyFromRows(data))
	if err != nil {
		log.Printf("BatchUserLog: CopyFrom error: %v", err)
		return
	}
	if int64(len(data)) != cnt {
		log.Printf("BatchUserLog: expected to insert %d rows, but inserted %d", len(data), cnt)
		return
	}
}

// BatchInsertCustomers выполняет массовую вставку клиентов
func BatchInsertCustomers(count int) error {
	log.Printf("Начало вставки %d клиентов...", count)
	start := time.Now()

	batchSize := 10000
	for i := 0; i < count; i += batchSize {
		end := i + batchSize
		if end > count {
			end = count
		}

		data := make([][]any, end-i)
		for j := i; j < end; j++ {
			birthdate := time.Date(1950+rand.Intn(60), time.Month(1+rand.Intn(12)), 1+rand.Intn(28), 0, 0, 0, 0, time.UTC)
			data[j-i] = []any{
				birthdate,
				fmt.Sprintf("Customer %d", j+1),
				fmt.Sprintf("customer%d@example.com", j+1),
				fmt.Sprintf("+992%09d", j+1),
				fmt.Sprintf("Address %d, Street %d", j+1, rand.Intn(100)),
				fmt.Sprintf("AA%07d", j+1),
			}
		}

		cnt, err := masterDB.CopyFrom(
			context.Background(),
			pgx.Identifier{"customers"},
			[]string{"birthdate", "name", "email", "phone", "address", "passport"},
			pgx.CopyFromRows(data),
		)
		if err != nil {
			return fmt.Errorf("CopyFrom customers error: %w", err)
		}
		if int64(len(data)) != cnt {
			return fmt.Errorf("expected to insert %d rows, but inserted %d", len(data), cnt)
		}

		if (i+batchSize)%100000 == 0 || end == count {
			log.Printf("Вставлено клиентов: %d/%d", end, count)
		}
	}

	log.Printf("Вставка %d клиентов завершена за %v", count, time.Since(start))
	return nil
}

// BatchInsertAccounts выполняет массовую вставку счетов
func BatchInsertAccounts(count int, maxCustomerID int64) error {
	log.Printf("Начало вставки %d счетов...", count)
	start := time.Now()

	currencies := []string{"USD", "EUR", "UZS", "RUB", "TJS"}
	accountTypes := []string{"checking", "savings", "credit", "investment"}

	batchSize := 10000
	for i := 0; i < count; i += batchSize {
		end := i + batchSize
		if end > count {
			end = count
		}

		data := make([][]any, end-i)
		for j := i; j < end; j++ {
			customerID := int64(1 + rand.Intn(int(maxCustomerID)))
			currencyCode := currencies[rand.Intn(len(currencies))]
			balance := float64(rand.Intn(1000000)) + rand.Float64()*100
			accountType := accountTypes[rand.Intn(len(accountTypes))]

			data[j-i] = []any{
				customerID,
				currencyCode,
				balance,
				fmt.Sprintf("Holder %d", customerID),
				accountType,
			}
		}

		cnt, err := masterDB.CopyFrom(
			context.Background(),
			pgx.Identifier{"accounts"},
			[]string{"customer_id", "currency_code", "balance", "account_holder", "account_type"},
			pgx.CopyFromRows(data),
		)
		if err != nil {
			return fmt.Errorf("CopyFrom accounts error: %w", err)
		}
		if int64(len(data)) != cnt {
			return fmt.Errorf("expected to insert %d rows, but inserted %d", len(data), cnt)
		}

		if (i+batchSize)%100000 == 0 || end == count {
			log.Printf("Вставлено счетов: %d/%d", end, count)
		}
	}

	log.Printf("Вставка %d счетов завершена за %v", count, time.Since(start))
	return nil
}

// BatchInsertPaymentOrders выполняет массовую вставку платежных ордеров в hypertable payment_orders
func BatchInsertPaymentOrders(count int, maxAccountID int64) error {
	log.Printf("Начало вставки %d платежных ордеров...", count)
	start := time.Now()

	currencies := []string{"USD", "EUR", "UZS", "RUB", "TJS"}
	paymentMethods := []string{"card", "bank_transfer", "cash", "wallet", "swift"}
	statuses := []int16{1, 2, 3, 4, 5} // pending, processing, completed, failed, cancelled

	// Диапазон дат - последний год
	yearAgo := time.Now().AddDate(-1, 0, 0)
	now := time.Now()
	daysDiff := int(now.Sub(yearAgo).Hours() / 24)

	batchSize := 10000
	for i := 0; i < count; i += batchSize {
		end := i + batchSize
		if end > count {
			end = count
		}

		data := make([][]any, end-i)
		for j := i; j < end; j++ {
			// Случайная дата создания за последний год
			randomDays := rand.Intn(daysDiff)
			createdAt := yearAgo.AddDate(0, 0, randomDays).Add(time.Duration(rand.Intn(86400)) * time.Second)

			// order_date - дата для партиционирования (только дата без времени)
			orderDate := createdAt.Truncate(24 * time.Hour)

			// Дата исполнения - от 0 до 30 дней после создания
			scheduledDate := createdAt.AddDate(0, 0, rand.Intn(31))

			senderAccountID := int64(1 + rand.Intn(int(maxAccountID)))
			beneficiaryAccountID := int64(1 + rand.Intn(int(maxAccountID)))
			// Убедимся что отправитель и получатель разные
			for beneficiaryAccountID == senderAccountID && maxAccountID > 1 {
				beneficiaryAccountID = int64(1 + rand.Intn(int(maxAccountID)))
			}

			currencyCode := currencies[rand.Intn(len(currencies))]
			amount := float64(rand.Intn(100000)) + rand.Float64()*100
			status := statuses[rand.Intn(len(statuses))]
			referenceNumber := fmt.Sprintf("REF-%d-%d", createdAt.Unix(), j+1)
			paymentMethod := paymentMethods[rand.Intn(len(paymentMethods))]
			updatedAt := createdAt.Add(time.Duration(rand.Intn(3600)) * time.Second)

			data[j-i] = []any{
				createdAt,
				updatedAt,
				scheduledDate,
				orderDate,
				senderAccountID,
				beneficiaryAccountID,
				amount,
				status,
				currencyCode,
				referenceNumber,
				paymentMethod,
			}
		}

		cnt, err := masterDB.CopyFrom(
			context.Background(),
			pgx.Identifier{"payment_orders"},
			[]string{"created_at", "updated_at", "scheduled_execution_date", "order_date", "sender_account_id",
				"beneficiary_account_id", "amount", "status", "currency_code", "reference_number", "payment_method"},
			pgx.CopyFromRows(data),
		)
		if err != nil {
			return fmt.Errorf("CopyFrom payment_orders error: %w", err)
		}
		if int64(len(data)) != cnt {
			return fmt.Errorf("expected to insert %d rows, but inserted %d", len(data), cnt)
		}

		if (i+batchSize)%100000 == 0 || end == count {
			log.Printf("Вставлено платежных ордеров: %d/%d", end, count)
		}
	}

	log.Printf("Вставка %d платежных ордеров завершена за %v", count, time.Since(start))
	return nil
}

// doSingleTransfer выполняет одну транзакцию перевода между счетами
func doSingleTransfer(ctx context.Context, tx pgx.Tx, senderID, beneficiaryID int64, amount float64, currency string) error {
	// Проверим, что счета разные
	if senderID == beneficiaryID {
		return fmt.Errorf("sender and beneficiary are the same: %d", senderID)
	}

	// Обновляем балансы
	cmd, err := tx.Exec(ctx, `
        UPDATE accounts
        SET balance = balance - $1
        WHERE id = $2 AND currency_code = $3 AND balance >= $1
    `, amount, senderID, currency)
	if err != nil {
		return fmt.Errorf("debit sender: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("insufficient funds or account not found for sender: %d", senderID)
	}

	cmd, err = tx.Exec(ctx, `
        UPDATE accounts
        SET balance = balance + $1
        WHERE id = $2 AND currency_code = $3
    `, amount, beneficiaryID, currency)
	if err != nil {
		return fmt.Errorf("credit beneficiary: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("account not found for beneficiary: %d", beneficiaryID)
	}

	// Вставляем запись в payment_orders (одна операция)
	createdAt := time.Now()
	orderDate := createdAt.Truncate(24 * time.Hour)
	scheduledDate := createdAt.AddDate(0, 0, rand.Intn(3)) // до 3 дней
	status := int16(3)                                     // допустим 'completed'
	referenceNumber := fmt.Sprintf("TX-%d-%d", createdAt.UnixNano(), rand.Intn(1_000_000))
	paymentMethod := "transfer"

	cmd, err = tx.Exec(ctx, `
        INSERT INTO payment_orders(
            created_at, updated_at, scheduled_execution_date, order_date,
            sender_account_id, beneficiary_account_id,
            amount, status, currency_code, reference_number, payment_method
        )
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    `, createdAt, createdAt, scheduledDate, orderDate,
		senderID, beneficiaryID,
		amount, status, currency, referenceNumber, paymentMethod)
	if err != nil {
		return fmt.Errorf("insert payment_order: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("failed to insert payment_order")
	}

	return nil
}

// workerTransactions — воркер, который выполняет много транзакций подряд
func workerTransactions(id int, totalOps int, maxAccountID int64, currencies []string) {
	ctx := context.Background()
	for i := 0; i < totalOps; i++ {
		// выбираем случайные счета и параметры
		senderID := int64(1 + rand.Intn(int(maxAccountID)))
		beneficiaryID := int64(1 + rand.Intn(int(maxAccountID)))
		amount := float64(1+rand.Intn(10_000)) + rand.Float64()
		currency := currencies[rand.Intn(len(currencies))]

		err := func() error {
			tx, err := masterDB.BeginTx(ctx, pgx.TxOptions{})
			if err != nil {
				return fmt.Errorf("begin tx: %w", err)
			}
			defer tx.Rollback(ctx) // безопасный rollback, commit его переопределит

			if err := doSingleTransfer(ctx, tx, senderID, beneficiaryID, amount, currency); err != nil {
				return err
			}

			if err := tx.Commit(ctx); err != nil {
				return fmt.Errorf("commit: %w", err)
			}
			return nil
		}()
		if err != nil {
			// для нагрузки можно просто логировать и продолжать
			log.Printf("worker %d, op %d error: %v", id, i, err)
		}

		if (i+1)%1000 == 0 {
			log.Printf("worker %d finished %d operations", id, i+1)
		}
	}
	log.Printf("worker %d finished all %d operations", id, totalOps)
}

// RunParallelTransactions запускает несколько горутин с транзакционными операциями
func RunParallelTransactions(workers, opsPerWorker int, maxAccountID int64) {
	log.Printf("Starting %d workers, %d ops each...", workers, opsPerWorker)
	currencies := []string{"USD", "EUR", "UZS", "RUB", "TJS"}

	done := make(chan struct{}, workers)
	for w := 0; w < workers; w++ {
		go func(id int) {
			workerTransactions(id, opsPerWorker, maxAccountID, currencies)
			done <- struct{}{}
		}(w + 1)
	}

	for i := 0; i < workers; i++ {
		<-done
	}
	log.Printf("All %d workers finished", workers)
}

func run1(ctx context.Context, senderID int64, beneficiaryID int64, amount float64, currency string) {
	tm := time.Now()
	err := func() error {
		tx, err := masterDB.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback(ctx) // безопасный rollback, commit его переопределит

		if err := doSingleTransfer(ctx, tx, senderID, beneficiaryID, amount, currency); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		return nil
	}()
	if err != nil {
		// для нагрузки можно просто логировать и продолжать
		log.Printf("worker %d, op %f error: %v", senderID, amount, err)
	}
	log.Printf("worker %d finished all %f operations in %v", senderID, amount, time.Since(tm))
}
