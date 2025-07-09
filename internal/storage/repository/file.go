package repository

import (
	"os"
)

type FileRepository struct{}

func NewFileRepository() *FileRepository {
	return &FileRepository{}
}

func (r *FileRepository) Save(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

func (r *FileRepository) Remove(path string) error {
	return os.Remove(path)
}
