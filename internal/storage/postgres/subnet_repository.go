package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/netip"

	// Register the pgx sql driver.
	_ "github.com/jackc/pgx/v5/stdlib"
)

type SubnetRepository struct {
	dataSourceName string
	db             *sql.DB
}

var ErrNotConnected = errors.New("database is not connected")

type ListType string

const (
	WhiteList ListType = "white_list"
	BlackList ListType = "black_list"
)

func New(dataSourceName string) *SubnetRepository {
	return &SubnetRepository{dataSourceName: dataSourceName}
}

func (s *SubnetRepository) Connect() error {
	if s.db != nil {
		return s.db.PingContext(context.Background())
	}
	if s.dataSourceName == "" {
		return fmt.Errorf("%w: empty dsn", ErrNotConnected)
	}

	db, err := sql.Open("pgx", s.dataSourceName)
	if err != nil {
		return err
	}
	if err := db.PingContext(context.Background()); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}

	s.db = db
	return nil
}

func (s *SubnetRepository) AddToWhiteList(ctx context.Context, prefix netip.Prefix) error {
	return s.add(ctx, WhiteList, prefix)
}

func (s *SubnetRepository) DeleteFromWhiteList(ctx context.Context, prefix netip.Prefix) error {
	return s.delete(ctx, WhiteList, prefix)
}

func (s *SubnetRepository) AddToBlackList(ctx context.Context, prefix netip.Prefix) error {
	return s.add(ctx, BlackList, prefix)
}

func (s *SubnetRepository) DeleteFromBlackList(ctx context.Context, prefix netip.Prefix) error {
	return s.delete(ctx, BlackList, prefix)
}

func (s *SubnetRepository) add(ctx context.Context, list ListType, prefix netip.Prefix) error {
	query := fmt.Sprintf("insert into %s (cidr) values ($1::cidr) on conflict do nothing", list)
	if _, err := s.db.ExecContext(ctx, query, prefix.Masked().String()); err != nil {
		return fmt.Errorf("add %q to %s: %w", prefix, list, err)
	}
	return nil
}

func (s *SubnetRepository) delete(ctx context.Context, list ListType, prefix netip.Prefix) error {
	query := fmt.Sprintf("delete from %s where cidr = $1::cidr", list)
	if _, err := s.db.ExecContext(ctx, query, prefix.Masked().String()); err != nil {
		return fmt.Errorf("delete %q from %s: %w", prefix, list, err)
	}
	return nil
}

func (s *SubnetRepository) GetWhiteList(ctx context.Context) ([]netip.Prefix, error) {
	return s.GetAll(ctx, WhiteList)
}

func (s *SubnetRepository) GetBlackList(ctx context.Context) ([]netip.Prefix, error) {
	return s.GetAll(ctx, BlackList)
}

func (s *SubnetRepository) GetAll(ctx context.Context, list ListType) ([]netip.Prefix, error) {
	query := fmt.Sprintf("select cidr::text from %s", list)
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get all from %s: %w", list, err)
	}
	defer rows.Close()

	var prefixes []netip.Prefix
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan %s row: %w", list, err)
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %q from %s: %w", raw, list, err)
		}
		prefixes = append(prefixes, prefix)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s rows: %w", list, err)
	}
	return prefixes, nil
}
