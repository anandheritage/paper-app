package usecase

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/paper-app/backend/internal/domain"
)

var (
	ErrPaperNotFound     = errors.New("paper not found")
	ErrPaperAlreadySaved = errors.New("paper already saved to library")
	ErrPaperNotInLibrary = errors.New("paper not in library")
)

type LibraryUsecase struct {
	userPaperRepo domain.UserPaperRepository
	paperRepo     domain.PaperRepository
}

func NewLibraryUsecase(userPaperRepo domain.UserPaperRepository, paperRepo domain.PaperRepository) *LibraryUsecase {
	return &LibraryUsecase{
		userPaperRepo: userPaperRepo,
		paperRepo:     paperRepo,
	}
}

type LibraryResult struct {
	Papers []*domain.UserPaper `json:"papers"`
	Total  int                 `json:"total"`
	Offset int                 `json:"offset"`
	Limit  int                 `json:"limit"`
}

func (u *LibraryUsecase) GetLibrary(userID uuid.UUID, status string, limit, offset int) (*LibraryResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	papers, total, err := u.userPaperRepo.GetByUser(userID, status, nil, limit, offset)
	if err != nil {
		return nil, err
	}

	return &LibraryResult{
		Papers: papers,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}, nil
}

func (u *LibraryUsecase) GetBookmarks(userID uuid.UUID, limit, offset int) (*LibraryResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	bookmarked := true
	papers, total, err := u.userPaperRepo.GetByUser(userID, "", &bookmarked, limit, offset)
	if err != nil {
		return nil, err
	}

	return &LibraryResult{
		Papers: papers,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}, nil
}

func (u *LibraryUsecase) SavePaper(userID, paperID uuid.UUID) (*domain.UserPaper, error) {
	paper, err := u.paperRepo.GetByID(paperID)
	if err != nil {
		return nil, err
	}
	if paper == nil {
		return nil, ErrPaperNotFound
	}

	existing, err := u.userPaperRepo.GetByUserAndPaper(userID, paperID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	userPaper := &domain.UserPaper{
		UserID:   userID,
		PaperID:  paperID,
		Status:   domain.StatusSaved,
		SavedAt:  time.Now(),
		Paper:    paper,
	}

	if err := u.userPaperRepo.Create(userPaper); err != nil {
		return nil, err
	}

	return userPaper, nil
}

func (u *LibraryUsecase) RemovePaper(userID, paperID uuid.UUID) error {
	existing, err := u.userPaperRepo.GetByUserAndPaper(userID, paperID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrPaperNotInLibrary
	}

	return u.userPaperRepo.Delete(userID, paperID)
}

type UpdatePaperInput struct {
	Status          *string `json:"status,omitempty"`
	ReadingProgress *int    `json:"reading_progress,omitempty"`
	Notes           *string `json:"notes,omitempty"`
}

func (u *LibraryUsecase) UpdatePaper(userID, paperID uuid.UUID, input *UpdatePaperInput) (*domain.UserPaper, error) {
	userPaper, err := u.userPaperRepo.GetByUserAndPaper(userID, paperID)
	if err != nil {
		return nil, err
	}
	if userPaper == nil {
		return nil, ErrPaperNotInLibrary
	}

	if input.Status != nil {
		userPaper.Status = *input.Status
	}
	if input.ReadingProgress != nil {
		userPaper.ReadingProgress = *input.ReadingProgress
		now := time.Now()
		userPaper.LastReadAt = &now
	}
	if input.Notes != nil {
		userPaper.Notes = *input.Notes
	}

	if err := u.userPaperRepo.Update(userPaper); err != nil {
		return nil, err
	}

	return userPaper, nil
}

func (u *LibraryUsecase) BookmarkPaper(userID, paperID uuid.UUID) (*domain.UserPaper, error) {
	userPaper, err := u.userPaperRepo.GetByUserAndPaper(userID, paperID)
	if err != nil {
		return nil, err
	}

	if userPaper == nil {
		// Save and bookmark
		paper, err := u.paperRepo.GetByID(paperID)
		if err != nil {
			return nil, err
		}
		if paper == nil {
			return nil, ErrPaperNotFound
		}

		userPaper = &domain.UserPaper{
			UserID:       userID,
			PaperID:      paperID,
			Status:       domain.StatusSaved,
			IsBookmarked: true,
			SavedAt:      time.Now(),
			Paper:        paper,
		}

		if err := u.userPaperRepo.Create(userPaper); err != nil {
			return nil, err
		}
	} else {
		userPaper.IsBookmarked = true
		if err := u.userPaperRepo.Update(userPaper); err != nil {
			return nil, err
		}
	}

	return userPaper, nil
}

func (u *LibraryUsecase) UnbookmarkPaper(userID, paperID uuid.UUID) error {
	userPaper, err := u.userPaperRepo.GetByUserAndPaper(userID, paperID)
	if err != nil {
		return err
	}
	if userPaper == nil {
		return ErrPaperNotInLibrary
	}

	userPaper.IsBookmarked = false
	return u.userPaperRepo.Update(userPaper)
}
