/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fsblkstorage

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
)

////  WRITER ////
type blockfileWriter struct {
	filePath string
	file     *os.File
}

func newBlockfileWriter(filePath string) (*blockfileWriter, error) {
	logger.Info("==newBlockfileWriter===")
	writer := &blockfileWriter{filePath: filePath}
	return writer, writer.open()
}

func (w *blockfileWriter) truncateFile(targetSize int) error {
	logger.Info("==blockfileWriter==truncateFile=")
	fileStat, err := w.file.Stat()
	if err != nil {
		return err
	}
	if fileStat.Size() > int64(targetSize) {
		w.file.Truncate(int64(targetSize))
	}
	return nil
}

func (w *blockfileWriter) append(b []byte, sync bool) error {
	logger.Info("==blockfileWriter==append=")
	_, err := w.file.Write(b)
	if err != nil {
		return err
	}
	if sync {
		return w.file.Sync()
	}
	return nil
}

func (w *blockfileWriter) open() error {
	logger.Info("==blockfileWriter==open=")
	file, err := os.OpenFile(w.filePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return errors.Wrapf(err, "error opening block file writer for file %s", w.filePath)
	}
	w.file = file
	return nil
}

func (w *blockfileWriter) close() error {
	logger.Info("==blockfileWriter==close=")
	return errors.WithStack(w.file.Close())
}

////  READER ////
type blockfileReader struct {
	file *os.File
}

func newBlockfileReader(filePath string) (*blockfileReader, error) {
	logger.Info("==newBlockfileReader=")
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0600)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening block file reader for file %s", filePath)
	}
	reader := &blockfileReader{file}
	return reader, nil
}

func (r *blockfileReader) read(offset int, length int) ([]byte, error) {
	logger.Info("====blockfileReader====read=")
	b := make([]byte, length)
	_, err := r.file.ReadAt(b, int64(offset))
	if err != nil {
		return nil, errors.Wrapf(err, "error reading block file for offset %d and length %d", offset, length)
	}
	return b, nil
}

func (r *blockfileReader) close() error {
	logger.Info("====blockfileReader====close=")
	return errors.WithStack(r.file.Close())
}
