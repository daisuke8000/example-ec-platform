package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

type CategoryUseCase interface {
	CreateCategory(ctx context.Context, input CreateCategoryInput) (*domain.Category, error)
	GetCategory(ctx context.Context, id uuid.UUID) (*domain.Category, error)
	ListCategories(ctx context.Context, parentID *uuid.UUID) ([]*domain.Category, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, input UpdateCategoryInput) (*domain.Category, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
}

type CreateCategoryInput struct {
	Name        string
	Description *string
	ParentID    *uuid.UUID
}

type UpdateCategoryInput struct {
	Name        *string
	Description *string
	ParentID    *uuid.UUID
}

type categoryUseCase struct {
	repo domain.CategoryRepository
}

func NewCategoryUseCase(repo domain.CategoryRepository) CategoryUseCase {
	return &categoryUseCase{repo: repo}
}

func (uc *categoryUseCase) CreateCategory(ctx context.Context, input CreateCategoryInput) (*domain.Category, error) {
	if input.ParentID != nil {
		if _, err := uc.repo.FindByID(ctx, *input.ParentID); err != nil {
			return nil, err
		}
	}

	exists, err := uc.repo.ExistsByNameAndParent(ctx, input.Name, input.ParentID, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrCategoryNameExists
	}

	category, err := domain.NewCategory(input.Name, input.Description, input.ParentID)
	if err != nil {
		return nil, err
	}

	if err := uc.repo.Create(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

func (uc *categoryUseCase) GetCategory(ctx context.Context, id uuid.UUID) (*domain.Category, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *categoryUseCase) ListCategories(ctx context.Context, parentID *uuid.UUID) ([]*domain.Category, error) {
	if parentID == nil {
		return uc.repo.FindAll(ctx)
	}
	return uc.repo.FindByParentID(ctx, parentID)
}

func (uc *categoryUseCase) UpdateCategory(ctx context.Context, id uuid.UUID, input UpdateCategoryInput) (*domain.Category, error) {
	category, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	name := category.Name
	if input.Name != nil {
		name = *input.Name
	}

	description := category.Description
	if input.Description != nil {
		description = input.Description
	}

	parentID := category.ParentID
	if input.ParentID != nil {
		parentID = input.ParentID
		if _, err := uc.repo.FindByID(ctx, *parentID); err != nil {
			return nil, err
		}
	}

	if name != category.Name || (parentID != nil && category.ParentID != nil && *parentID != *category.ParentID) {
		exists, err := uc.repo.ExistsByNameAndParent(ctx, name, parentID, &id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, domain.ErrCategoryNameExists
		}
	}

	if err := category.Update(name, description, parentID); err != nil {
		return nil, err
	}

	if err := uc.repo.Update(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

func (uc *categoryUseCase) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	return uc.repo.SoftDelete(ctx, id)
}
