package domain

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxCategoryNameLength = 255

type Category struct {
	ID          uuid.UUID
	Name        string
	Description *string
	ParentID    *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type CategoryRepository interface {
	Create(ctx context.Context, category *Category) error
	FindByID(ctx context.Context, id uuid.UUID) (*Category, error)
	FindByParentID(ctx context.Context, parentID *uuid.UUID) ([]*Category, error)
	FindAll(ctx context.Context) ([]*Category, error)
	Update(ctx context.Context, category *Category) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ExistsByNameAndParent(ctx context.Context, name string, parentID *uuid.UUID, excludeID *uuid.UUID) (bool, error)
}

func NewCategory(name string, description *string, parentID *uuid.UUID) (*Category, error) {
	if err := ValidateCategoryName(name); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &Category{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		ParentID:    parentID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func ValidateCategoryName(name string) error {
	if name == "" {
		return ErrEmptyCategoryName
	}
	if utf8.RuneCountInString(name) > MaxCategoryNameLength {
		return ErrCategoryNameTooLong
	}
	return nil
}

func (c *Category) ValidateParentID(parentID *uuid.UUID) error {
	if parentID != nil && *parentID == c.ID {
		return ErrSelfParentCategory
	}
	return nil
}

func (c *Category) IsDeleted() bool {
	return c.DeletedAt != nil
}

func (c *Category) Update(name string, description *string, parentID *uuid.UUID) error {
	if err := ValidateCategoryName(name); err != nil {
		return err
	}
	if err := c.ValidateParentID(parentID); err != nil {
		return err
	}

	c.Name = name
	c.Description = description
	c.ParentID = parentID
	c.UpdatedAt = time.Now().UTC()
	return nil
}
