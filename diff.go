package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/schollz/progressbar/v3"
)

type RowDiff struct {
	PrimaryKey int64             `json:"primary_key"`
	Columns    []string          `json:"columns"`
	Left       map[string]string `json:"left"`
	Right      map[string]string `json:"right"`
}

type diffStore struct {
	wLock sync.Mutex
	file  *os.File
}

func (s *diffStore) save(d *RowDiff) error {
	s.wLock.Lock()
	defer s.wLock.Unlock()
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = s.file.WriteString(string(data) + "\n")
	return err
}

func (s *diffStore) close() {
	s.file.Sync()
	s.file.Close()
}

type differ struct {
	worker     int
	left       *gorm.DB
	right      *gorm.DB
	leftTable  string
	rightTable string
	cols       []string
	primaryKey string
	segFrom    int64
	segTo      int64
	segStep    int64
	where      string
	dbRate     int64
	diffStore  *diffStore
}

func newDiffer(cfg *config) (*differ, error) {
	var err error
	d := &differ{
		worker:     cfg.Concurrency.Worker,
		leftTable:  cfg.Left.Table,
		rightTable: cfg.Right.Table,
		cols:       cfg.DiffColumns,
		primaryKey: cfg.PrimaryKey,
		segFrom:    cfg.Segment.From,
		segTo:      cfg.Segment.To,
		segStep:    cfg.Segment.Step,
		where:      cfg.Filter.Where,
		dbRate:     0,
		diffStore:  nil,
	}
	d.left, err = gorm.Open("mysql", cfg.Left.DSN)
	if err != nil {
		return nil, err
	}
	d.right, err = gorm.Open("mysql", cfg.Right.DSN)
	if err != nil {
		return nil, err
	}
	d.diffStore = &diffStore{
		wLock: sync.Mutex{},
	}
	d.diffStore.file, err = os.Create(cfg.Output.File)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *differ) diff() (err error) {
	crcDiff := func(from, to int64) (ok bool, err error) {
		genSql := func(table string) string {
			cols := []string{}
			for _, col := range d.cols {
				cols = append(cols, fmt.Sprintf("cast(`%s` as char)", col))
			}
			sql := fmt.Sprintf(
				"SELECT IFNULL(sum(crc32(concat(%s))), 0) as crc FROM `%s` WHERE (%s >= %d) AND (%s < %d) AND (%s)",
				strings.Join(cols, ","),
				table,
				d.primaryKey, from,
				d.primaryKey, to,
				d.where,
			)
			return sql
		}
		type crcRes struct {
			Crc int64
		}
		query := func(db *gorm.DB, table string) (res *crcRes, err error) {
			res = new(crcRes)
			row := db.Raw(genSql(table)).Row()
			err = row.Scan(&res.Crc)
			return res, err
		}
		var (
			left  *crcRes
			right *crcRes
		)
		g := sync.WaitGroup{}
		g.Add(1)
		go func() {
			defer g.Done()
			var innerErr error
			left, innerErr = query(d.left, d.leftTable)
			if innerErr != nil {
				err = innerErr
			}
		}()
		g.Add(1)
		go func() {
			defer g.Done()
			var innerErr error
			right, innerErr = query(d.right, d.rightTable)
			if innerErr != nil {
				err = innerErr
			}
		}()
		g.Wait()
		if err != nil {
			return
		}
		return left.Crc == right.Crc, nil
	}

	rowsDiff := func(from, to int64) (diffs []*RowDiff, err error) {
		genSql := func(table string) string {
			cols := []string{}
			for _, col := range d.cols {
				cols = append(cols, fmt.Sprintf("cast(`%s` as char)", col))
			}
			sql := fmt.Sprintf(
				"SELECT %s FROM `%s` WHERE (%s >= %d) AND (%s < %d) AND (%s)",
				strings.Join(cols, ","),
				table,
				d.primaryKey, from,
				d.primaryKey, to,
				d.where,
			)
			return sql
		}
		query := func(db *gorm.DB, table string) (res []map[string]string, err error) {
			res = make([]map[string]string, 0, 0)
			rows, err := db.Raw(genSql(table)).Rows()
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				row := make(map[string]string)
				cols := make([]string, len(d.cols))
				pointers := make([]interface{}, len(d.cols))
				for i, _ := range cols {
					pointers[i] = &cols[i]
				}
				rows.Scan(pointers...)
				for i, _ := range cols {
					row[d.cols[i]] = cols[i]
				}
				res = append(res, row)
			}
			//spew.Dump(res)
			return res, err
		}
		var (
			leftRows  []map[string]string
			rightRows []map[string]string
		)
		g := sync.WaitGroup{}
		g.Add(1)
		go func() {
			defer g.Done()
			var innerErr error
			leftRows, innerErr = query(d.left, d.leftTable)
			if innerErr != nil {
				err = innerErr
			}
		}()
		g.Add(1)
		go func() {
			defer g.Done()
			var innerErr error
			rightRows, innerErr = query(d.right, d.rightTable)
			if innerErr != nil {
				err = innerErr
			}
		}()
		g.Wait()
		if err != nil {
			return
		}
		compareRow := func(left, right map[string]string) (ok bool, cols []string) {
			if left == nil || right == nil {
				return false, d.cols
			}
			ok = true
			for _, col := range d.cols {
				if left[col] != right[col] {
					ok = false
					cols = append(cols, col)
				}
			}
			return ok, cols
		}
		diffs = make([]*RowDiff, 0, 0)
		for _, row := range leftRows {
			diff := &RowDiff{
				Left:  row,
				Right: nil,
			}
			pk, err := strconv.Atoi(row[d.primaryKey])
			if err != nil {
				return nil, err
			}
			diff.PrimaryKey = int64(pk)
			for _, rRow := range rightRows {
				if rRow[d.primaryKey] == row[d.primaryKey] {
					diff.Right = rRow
					break
				}
			}
			var ok bool
			ok, diff.Columns = compareRow(diff.Left, diff.Right)
			if !ok {
				diffs = append(diffs, diff)
			}
		}
		for _, row := range rightRows {
			exist := false
			for _, lRow := range leftRows {
				if row[d.primaryKey] == lRow[d.primaryKey] {
					exist = true
					break
				}
			}
			if !exist {
				diff := &RowDiff{
					Left:  nil,
					Right: row,
				}
				pk, err := strconv.Atoi(row[d.primaryKey])
				if err != nil {
					return nil, err
				}
				diff.PrimaryKey = int64(pk)
				var ok bool
				ok, diff.Columns = compareRow(diff.Left, diff.Right)
				if !ok {
					diffs = append(diffs, diff)
				}
			}
		}
		return diffs, nil
	}
	diff := func(from, to int64) (ok bool, diffs []*RowDiff, err error) {
		match, err := crcDiff(from, to)
		if err != nil {
			return false, nil, err
		}
		if match {
			return true, nil, nil
		}
		diffs, err = rowsDiff(from, to)
		if err != nil {
			return false, nil, err
		}
		log.Warnf("[table-diff:differ] there are some differences in the current segment, seg_from:%d seg_to:%d diff:%d", from, to, len(diffs))
		return false, diffs, err
	}
	type task struct {
		from int64
		to   int64
	}
	bar := progressbar.Default((d.segTo - d.segFrom) / d.segStep)
	diffTaskChan := make(chan task)
	errChan := make(chan error)
	stopChan := make(chan struct{})
	g := sync.WaitGroup{}
	for i := 0; i < d.worker; i++ {
		g.Add(1)
		go func() {
			defer g.Done()
			for {
				select {
				case <-stopChan:
					return
				case task, ok := <-diffTaskChan:
					if !ok {
						return
					}
					ok, diffs, err := diff(task.from, task.to)
					if err != nil {
						log.Errorf("[table-diff:differ] some bad exception raises when this app is trying to run a diff task, seg_from:%d seg_to:%d err:%s", task.from, task.to, err)
						errChan <- err
						return
					}
					if ok {
						log.Debugf("[table-diff:differ] there aren't any differences in the current segment, seg_from:%d seg_to:%d", task.from, task.to)
						bar.Add(1)
						continue
					}
					for _, diff := range diffs {
						if err := d.diffStore.save(diff); err != nil {
							log.Errorf("[table-diff:differ] some bad exception raises when this app is trying to save a diff task, diff:%+v err:%s", *diff, err)
							errChan <- err
							return
						}
					}

				}
			}
		}()
	}
	go func() {
		err = <-errChan
		close(stopChan)
	}()
	for _, step := range StepsFrom(d.segFrom, d.segTo+1, d.segStep) {
		diffTaskChan <- task{
			from: step.Head,
			to:   step.Tail,
		}
	}
	close(diffTaskChan)
	g.Wait()
	d.diffStore.close()
	if err != nil {
		return err
	}
	return nil
}
