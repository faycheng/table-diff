package main

import (
	"fmt"
	"os"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/pelletier/go-toml/v2"
	uuid "github.com/satori/go.uuid"
)

type config struct {
	PrimaryKey  string
	DiffColumns []string
	Left        struct {
		DSN   string
		Table string
	}
	Right struct {
		DSN   string
		Table string
	}
	Segment struct {
		From int64
		To   int64
		Step int64
	}
	Filter struct {
		Where string
	}
	Concurrency struct {
		Worker int
	}
	Output struct {
		File string
	}
}

func (c *config) checkAndFix() error {
	if c.Left.DSN == "" {
		return fmt.Errorf("the configuration item(%s) of the left table is missing", "dsn")
	}
	if c.Left.Table == "" {
		return fmt.Errorf("the configuration item(%s) of the left table is missing", "table")
	}
	if c.Right.DSN == "" {
		return fmt.Errorf("the configuration item(%s) of the right table is missing", "dsn")
	}
	if c.Right.Table == "" {
		return fmt.Errorf("the configuration item(%s) of the right table is missing", "table")
	}
	if c.PrimaryKey == "" {
		c.PrimaryKey = "id"
	}
	ldb, err := gorm.Open("mysql", c.Left.DSN)
	if err != nil {
		return fmt.Errorf("bad configuration item(%s) of the left table", "dsn")
	}
	type col struct {
		Field   string `json:"Field"`
		Type    string
		Null    string
		Key     string
		Default string
		Extra   string
	}
	listColumns := func(db *gorm.DB, table string) (cols []*col, err error) {
		cols = make([]*col, 0, 0)
		rows, err := db.Raw(fmt.Sprintf("SHOW COLUMNS FROM `%s`", table)).Rows()
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			col := new(col)
			rows.Scan(&col.Field, &col.Type, &col.Null, &col.Key, &col.Default, &col.Extra)
			cols = append(cols, col)
		}
		return cols, err
	}
	leftCols, err := listColumns(ldb, c.Left.Table)
	if err != nil {
		return err
	}
	for _, col := range leftCols {
		if col.Field == c.PrimaryKey {
			if col.Key != "PRI" {
				return fmt.Errorf("bad configuration item(%s) of the left table", "primary key")
			}
		}
	}
	if len(c.DiffColumns) == 0 {
		for _, col := range leftCols {
			c.DiffColumns = append(c.DiffColumns, col.Field)
		}
	}
	for _, col := range c.DiffColumns {
		exist := false
		for _, col1 := range leftCols {
			if col1.Field == col {
				exist = true
				break
			}
		}
		if !exist {
			return fmt.Errorf("bad configuration item(%s) of the left table", "diff cols")
		}
	}
	rdb, err := gorm.Open("mysql", c.Right.DSN)
	if err != nil {
		return fmt.Errorf("bad configuration item(%s) of the right table", "dsn")
	}
	rightCols, err := listColumns(rdb, c.Right.Table)
	if err != nil {
		return err
	}
	for _, col := range rightCols {
		if col.Field == c.PrimaryKey {
			if col.Key != "PRI" {
				return fmt.Errorf("bad configuration item(%s) of the right table", "primary key")
			}
		}
	}
	for _, col := range c.DiffColumns {
		exist := false
		for _, col1 := range rightCols {
			if col1.Field == col {
				exist = true
				break
			}
		}
		if !exist {
			return fmt.Errorf("bad configuration item(%s) of the right table", "diff cols")
		}
	}
	contain := false
	for _, col := range c.DiffColumns {
		if col == c.PrimaryKey {
			contain = true
		}
	}
	if !contain {
		c.DiffColumns = append(c.DiffColumns, c.PrimaryKey)
	}
	maxKey := func(db *gorm.DB, table string) (int64, error) {
		id := int64(0)
		row := db.Raw(fmt.Sprintf("SELECT max(%s) FROM `%s`", c.PrimaryKey, table)).Row()
		err := row.Scan(&id)
		if err != nil {
			return 0, err
		}
		return id, err
	}
	lmax, err := maxKey(ldb, c.Left.Table)
	if err != nil {
		return err
	}
	rmax, err := maxKey(rdb, c.Right.Table)
	if err != nil {
		return err
	}
	if c.Segment.To == 0 {
		c.Segment.To = lmax
		if rmax > lmax {
			c.Segment.To = rmax
		}
	}
	if c.Segment.Step == 0 {
		c.Segment.Step = 1000
	}
	if c.Concurrency.Worker == 0 {
		c.Concurrency.Worker = 1
	}
	if c.Filter.Where == "" {
		c.Filter.Where = "true"
	}
	if c.Output.File == "" {
		c.Output.File = fmt.Sprintf("/tmp/table-diff-%s.diff", uuid.NewV4().String())
	}
	return nil
}

func readConf(conf string) (*config, error) {
	if exist, err := fileExist(conf); err != nil || !exist {
		if err != nil {
			return nil, err
		}
		if !exist {
			return nil, fmt.Errorf("config file doesn't exist: %s", conf)
		}
	}
	data, err := os.ReadFile(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to read data of config file from disk: %s %s", conf, err)
	}
	cfg := &config{}
	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal the config file: %s %s", conf, err)
	}
	if err = cfg.checkAndFix(); err != nil {
		return nil, err
	}
	return cfg, nil
}
