package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

type ProductUseCase interface {
	CreateProduct(ctx context.Context, input CreateProductInput) (*domain.Product, error)
	GetProduct(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	GetProductWithSKUs(ctx context.Context, id uuid.UUID) (*domain.ProductWithSKUs, error)
	ListProducts(ctx context.Context, filter domain.ProductFilter, pagination domain.Pagination) ([]*domain.Product, int64, error)
	UpdateProduct(ctx context.Context, id uuid.UUID, input UpdateProductInput) (*domain.Product, error)
	UpdateProductStatus(ctx context.Context, id uuid.UUID, status domain.ProductStatus) error
	DeleteProduct(ctx context.Context, id uuid.UUID) error
}

type CreateProductInput struct {
	Name        string
	Description *string
	CategoryID  *uuid.UUID
}

type UpdateProductInput struct {
	Name        *string
	Description *string
	CategoryID  *uuid.UUID
}

type productUseCase struct {
	productRepo  domain.ProductRepository
	categoryRepo domain.CategoryRepository
}

func NewProductUseCase(productRepo domain.ProductRepository, categoryRepo domain.CategoryRepository) ProductUseCase {
	return &productUseCase{
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
	}
}

func (uc *productUseCase) CreateProduct(ctx context.Context, input CreateProductInput) (*domain.Product, error) {
	if input.CategoryID != nil {
		if _, err := uc.categoryRepo.FindByID(ctx, *input.CategoryID); err != nil {
			return nil, err
		}
	}

	product, err := domain.NewProduct(input.Name, input.Description, input.CategoryID)
	if err != nil {
		return nil, err
	}

	if err := uc.productRepo.Create(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}

func (uc *productUseCase) GetProduct(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	return uc.productRepo.FindByID(ctx, id)
}

func (uc *productUseCase) GetProductWithSKUs(ctx context.Context, id uuid.UUID) (*domain.ProductWithSKUs, error) {
	return uc.productRepo.FindByIDWithSKUs(ctx, id)
}

func (uc *productUseCase) ListProducts(ctx context.Context, filter domain.ProductFilter, pagination domain.Pagination) ([]*domain.Product, int64, error) {
	return uc.productRepo.List(ctx, filter, pagination)
}

func (uc *productUseCase) UpdateProduct(ctx context.Context, id uuid.UUID, input UpdateProductInput) (*domain.Product, error) {
	product, err := uc.productRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	name := product.Name
	if input.Name != nil {
		name = *input.Name
	}

	description := product.Description
	if input.Description != nil {
		description = input.Description
	}

	categoryID := product.CategoryID
	if input.CategoryID != nil {
		categoryID = input.CategoryID
		if _, err := uc.categoryRepo.FindByID(ctx, *categoryID); err != nil {
			return nil, err
		}
	}

	if err := product.Update(name, description, categoryID); err != nil {
		return nil, err
	}

	if err := uc.productRepo.Update(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}

func (uc *productUseCase) UpdateProductStatus(ctx context.Context, id uuid.UUID, status domain.ProductStatus) error {
	if err := domain.ValidateProductStatus(status); err != nil {
		return err
	}
	return uc.productRepo.UpdateStatus(ctx, id, status)
}

func (uc *productUseCase) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	return uc.productRepo.SoftDeleteWithSKUs(ctx, id)
}
