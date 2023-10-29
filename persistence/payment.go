package persistence

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
)

type Payment struct {
	ID                          uint
	PoolID                      string
	Coin                        string
	Address                     string
	Amount                      float32
	TransactionConfirmationData string
	Created                     time.Time
}

type PaymentRepository struct {
	*sql.DB
}

func (r *PaymentRepository) Insert(payment Payment) error {
	query := "INSERT INTO payments(poolid, coin, address, amount, transactionconfirmationdata, created) "
	query = query + "VALUES(?, ?, ?, ?, ?, ?)"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(&payment.PoolID, &payment.Coin, &payment.Address, &payment.Amount,
		&payment.TransactionConfirmationData, &payment.Created)
	return err
}

func (r *PaymentRepository) InsertBatch(payments []Payment) error {
	txn, err := r.DB.Begin()
	if err != nil {
		return err
	}

	fields := pq.CopyIn("poolid", "coin", "address", "amount", "transactionconfirmationdata", "created")
	stmt, err := txn.Prepare(fields)
	if err != nil {
		return err
	}

	for _, payment := range payments {
		_, err = stmt.Exec(payment.PoolID, payment.Coin, payment.Address, payment.Amount,
			payment.TransactionConfirmationData, payment.Created)
		if err != nil {
			return err
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	err = stmt.Close()
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (r *PaymentRepository) PagePayments(poolID, miner string, page, pageSize int) ([]Payment, error) {
	query := "SELECT poolid, coin, address, amount, transactionconfirmationdata, created FROM payments WHERE poolid = ? "
	if miner != "" {
		query = query + " AND address = ? "
	}
	query = query + "ORDER BY created DESC OFFSET ? FETCH NEXT ? ROWS ONLY"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return nil, err
	}

	var payments []Payment
	var rows *sql.Rows
	if miner == "" {
		rows, err = stmt.Query(poolID, page, pageSize)
	} else {
		rows, err = stmt.Query(poolID, miner, page, pageSize)
	}
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var payment Payment

		err = rows.Scan(&payment.PoolID, &payment.Coin, &payment.Address, &payment.Amount,
			&payment.TransactionConfirmationData, &payment.Created)
		if err != nil {
			return payments, err
		}

		payments = append(payments, payment)
	}

	return payments, nil
}

func (r *PaymentRepository) PageMinerPaymentsByDay(poolID, miner string, page, pageSize int) ([]Payment, error) {
	query := "SELECT SUM(amount) AS amount, date_trunc('day', created) AS date FROM payments WHERE poolid = ? "
	if miner != "" {
		query = query + " AND address = ? "
	}
	query = query + "GROUP BY date ORDER BY date DESC OFFSET ? FETCH NEXT ? ROWS ONLY"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return nil, err
	}

	var payments []Payment
	var rows *sql.Rows
	if miner == "" {
		rows, err = stmt.Query(poolID, page, pageSize)
	} else {
		rows, err = stmt.Query(poolID, miner, page, pageSize)
	}
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var payment Payment

		err = rows.Scan(&payment.PoolID, &payment.Coin, &payment.Address, &payment.Amount,
			&payment.TransactionConfirmationData, &payment.Created)
		if err != nil {
			return payments, err
		}

		payments = append(payments, payment)
	}

	return payments, nil
}

func (r *PaymentRepository) PaymentsCount(poolID, miner string) (uint, error) {
	query := "SELECT COUNT(*) FROM payments WHERE poolid = ?"
	if miner != "" {
		query = query + " AND address = ? "
	}

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return 0, err
	}

	var count uint
	if miner == "" {
		err = stmt.QueryRow(poolID).Scan(&count)
	} else {
		err = stmt.QueryRow(poolID, miner).Scan(&count)
	}
	if err != nil {
		return 0, err
	}

	return count, nil
}

type PaymentsCountByDay map[string]uint

func (r *PaymentRepository) MinerPaymentsByDayCount(poolID, miner string) (PaymentsCountByDay, error) {
	paymentsByDay := make(PaymentsCountByDay)

	query := "SELECT COUNT(*) FROM (SELECT SUM(amount) AS amount, date_trunc('day', created) AS date "
	query = query + "FROM payments WHERE poolid = ? "
	query = query + "AND address = ? "
	query = query + "FROM GROUP BY date "
	query = query + "ORDER BY date DESC) s"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return nil, err
	}

	rows, err := stmt.Query(poolID, miner)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var date string
		var count uint

		err = rows.Scan(&count, &date)
		if err != nil {
			return nil, err
		}

		paymentsByDay[date] = count
	}

	return paymentsByDay, nil
}

func (r *PaymentRepository) MinerLastPayment(poolID, miner string) (*Payment, error) {
	query := "SELECT poolid, coin, address, amount, transactionconfirmationdata, created "
	query = query + "FROM payments WHERE poolid = ? AND address = ? ORDER BY created DESC LIMIT 1"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return nil, err
	}

	row := stmt.QueryRow(poolID, miner)
	if row == nil {
		return nil, nil
	}

	var payment Payment
	err = row.Scan(&payment.PoolID, &payment.Coin, &payment.Address,
		&payment.Amount, &payment.TransactionConfirmationData, &payment.Created)
	if err != nil {
		return nil, err
	}

	return &payment, nil
}