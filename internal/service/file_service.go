package service

import (
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
)

type FileService struct {
	FileRepository       *repository.FileRepository
	defaultUserIconPaths map[string]struct{}
}

func NewFileService(fileRepo *repository.FileRepository) (*FileService, error) {
	files, err := os.ReadDir(MediaFilesDirectory)
	if err != nil {
		return nil, apperrors.Internal("failed to read default user icon's dir", err)
	}

	svgFiles := make(map[string]struct{}, len(files))
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".svg" {
			svgFiles[file.Name()] = struct{}{}
		}
	}

	return &FileService{
		defaultUserIconPaths: svgFiles,
		FileRepository:       fileRepo,
	}, nil
}

func (s *FileService) IsDefaultUserIcon(file string) bool {
	_, ok := s.defaultUserIconPaths[file]
	return ok
}

var allowedImageExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".svg":  {},
	".png":  {},
}

const (
	mediaFilesDir       = "storage"
	adminUploadMediaDir = "admin_uploads"
	userUploadMediaDir  = "user_uploads"
	faqQuestionImages   = "faq_question"
	faqAnswerImages     = "faq_answer"
)

func (s *FileService) SaveAdminsFile(
	file *multipart.FileHeader,
) (string, error) {
	return s.saveFile(adminUploadMediaDir, file)
}

func (s *FileService) SaveUserProfilePicture(
	file *multipart.FileHeader,
) (string, error) {
	return s.saveFile(userUploadMediaDir, file)
}

func (s *FileService) SaveFAQQuestionImages(
	mediaUrl string,
	files []*multipart.FileHeader,
) ([]string, error) {
	filesPath := make([]string, len(files))

	for i, file := range files {
		path, err := s.saveFile(faqQuestionImages, file)
		if err != nil {
			return nil, err
		}
		filesPath[i] = mediaUrl + path
	}

	return filesPath, nil
}

func (s *FileService) SaveFAQAnswersImages(
	mediaUrl string,
	originalFileName string,
	data []byte,
) (string, error) {

	path, err := s.saveFileFromBytes(faqAnswerImages, originalFileName, data)
	if err != nil {
		return "", err
	}

	return mediaUrl + path, nil
}

const maxAllowedSize = 3 * 1024 * 1024 // 3 MB

func (s *FileService) saveFile(
	dir string,
	file *multipart.FileHeader,
) (string, error) {
	if file.Size > maxAllowedSize {
		return "", apperrors.BadRequest("file exceeds max allowed size")
	}

	extension := strings.ToLower(filepath.Ext(file.Filename))
	if _, ok := allowedImageExtensions[extension]; !ok {
		return "", apperrors.BadRequest("invalid file extension")
	}

	fileContent, err := file.Open()
	if err != nil {
		return "", apperrors.Internal("failed to open an image", err)
	}
	defer func() { _ = fileContent.Close() }()

	data, err := io.ReadAll(fileContent)
	if err != nil {
		return "", apperrors.Internal("failed to read an image", err)
	}

	fileName := uuid.New().String() + extension
	savePath := filepath.Join(mediaFilesDir, dir, fileName)

	if err = s.FileRepository.Save(savePath, data); err != nil {
		return "", apperrors.Internal("failed to save an image", err)
	}

	publicURL := "/" + filepath.Join(dir, fileName)

	return publicURL, nil
}

func (s *FileService) saveFileFromBytes(dir, originalFileName string, data []byte) (string, error) {

	if len(data) > maxAllowedSize {
		return "", apperrors.BadRequest("file exceeds max allowed size")
	}

	extension := strings.ToLower(filepath.Ext(originalFileName))
	if _, ok := allowedImageExtensions[extension]; !ok {
		return "", apperrors.BadRequest("invalid file extension")
	}

	fileName := uuid.New().String() + extension
	savePath := filepath.Join(mediaFilesDir, dir, fileName)

	if err := s.FileRepository.Save(savePath, data); err != nil {
		return "", apperrors.Internal("failed to save an image", err)
	}

	publicURL := "/" + filepath.Join(dir, fileName)

	return publicURL, nil
}

func (s *FileService) RemoveFileAdmin(
	fileName string,
) error {
	return s.RemoveFile(adminUploadMediaDir, fileName)
}

func (s *FileService) RemoveUserFile(
	fileName string,
) error {
	return s.RemoveFile(userUploadMediaDir, fileName)
}

func (s *FileService) RemoveFAQImages(
	questionFileNames []string,
	answerFileNames []string,
) error {

	for _, fn := range questionFileNames {
		err := s.RemoveFile(faqQuestionImages, fn)
		if err != nil {
			return err
		}
	}

	for _, fn := range answerFileNames {
		err := s.RemoveFile(faqAnswerImages, fn)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *FileService) RemoveFile(
	dir string,
	fileName string,
) error {
	path := filepath.Join(mediaFilesDir, fileName)
	cleanPath := filepath.Clean(path)

	expectedPrefix := filepath.Join(mediaFilesDir, dir)
	if !strings.HasPrefix(cleanPath, expectedPrefix) {
		return apperrors.BadRequest("invalid path: attempting directory traversal")
	}

	if _, ok := allowedImageExtensions[filepath.Ext(fileName)]; !ok {
		return apperrors.BadRequest("invalid file extension")
	}

	if err := s.FileRepository.Remove(cleanPath); err != nil {
		if os.IsNotExist(err) {
			return apperrors.NotFound("file not found")
		}

		return apperrors.Internal("failed to delete file", err)
	}

	return nil
}
