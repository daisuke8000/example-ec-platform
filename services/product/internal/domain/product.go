package domain

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxProductNameLength = 255

type ProductStatus int32

const (
	ProductStatusUnspecified ProductStatus = 0
	ProductStatusDraft       ProductStatus = 1
	ProductStatusPublished   ProductStatus = 2
	ProductStatusHidden      ProductStatus = 3
)

func (s ProductStatus) String() string {
	switch s {
	case ProductStatusDraft:
		return "DRAFT"
	case ProductStatusPublished:
		return "PUBLISHED"
	case ProductStatusHidden:
		return "HIDDEN"
	default:
		return "UNSPECIFIED"
	}
}

func (s ProductStatus) IsValid() bool {
	return s >= ProductStatusUnspecified && s <= ProductStatusHidden
}

type Product struct {
	ID          uuid.UUID
	Name        string
	Description *string
	CategoryID  *uuid.UUID
	Status      ProductStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type ProductWithSKUs struct {
	Product *Product
	SKUs    []*SKU
}

type ProductRepository interface {
	Create(ctx context.Context, product *Product) error
	FindByID(ctx context.Context, id uuid.UUID) (*Product, error)
	FindByIDWithSKUs(ctx context.Context, id uuid.UUID) (*ProductWithSKUs, error)
	List(ctx context.Context, filter ProductFilter, pagination Pagination) ([]*Product, int64, error)
	Update(ctx context.Context, product *Product) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status ProductStatus) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	SoftDeleteWithSKUs(ctx context.Context, id uuid.UUID) error
}

type ProductFilter struct {
	CategoryID *uuid.UUID
	Status     *ProductStatus
	Search     *string
}

type Pagination struct {
	PageSize  int32
	PageToken string
}

func NewProduct(name string, description *string, categoryID *uuid.UUID) (*Product, error) {
	if err := ValidateProductName(name); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &Product{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		CategoryID:  categoryID,
		Status:      ProductStatusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func ValidateProductName(name string) error {
	if name == "" {
		return ErrEmptyProductName
	}
	if utf8.RuneCountInString(name) > MaxProductNameLength {
		return ErrProductNameTooLong
	}
	return nil
}

func ValidateProductStatus(status ProductStatus) error {
	if !status.IsValid() {
		return ErrInvalidProductStatus
	}
	return nil
}

func (p *Product) IsDeleted() bool {
	return p.DeletedAt != nil
}

func (p *Product) IsPublished() bool {
	return p.Status == ProductStatusPublished
}

func (p *Product) Update(name string, description *string, categoryID *uuid.UUID) error {
	if err := ValidateProductName(name); err != nil {
		return err
	}

	p.Name = name
	p.Description = description
	p.CategoryID = categoryID
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *Product) SetStatus(status ProductStatus) error {
	if err := ValidateProductStatus(status); err != nil {
		return err
	}
	p.Status = status
	p.UpdatedAt = time.Now().UTC()
	return nil
}
