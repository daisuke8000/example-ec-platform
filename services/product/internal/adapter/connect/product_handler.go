package connect

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	productv1 "github.com/daisuke8000/example-ec-platform/gen/product/v1"
	"github.com/daisuke8000/example-ec-platform/gen/product/v1/productv1connect"
	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
	"github.com/daisuke8000/example-ec-platform/services/product/internal/usecase"
)

type ProductHandler struct {
	productv1connect.UnimplementedProductServiceHandler
	productUC  usecase.ProductUseCase
	skuUC      usecase.SKUUseCase
	categoryUC usecase.CategoryUseCase
}

func NewProductHandler(
	productUC usecase.ProductUseCase,
	skuUC usecase.SKUUseCase,
	categoryUC usecase.CategoryUseCase,
) *ProductHandler {
	return &ProductHandler{
		productUC:  productUC,
		skuUC:      skuUC,
		categoryUC: categoryUC,
	}
}

func (h *ProductHandler) CreateProduct(
	ctx context.Context,
	req *connect.Request[productv1.CreateProductRequest],
) (*connect.Response[productv1.CreateProductResponse], error) {
	input := usecase.CreateProductInput{
		Name: req.Msg.Name,
	}
	if req.Msg.Description != "" {
		input.Description = &req.Msg.Description
	}
	if req.Msg.CategoryId != nil {
		categoryID, err := uuid.Parse(*req.Msg.CategoryId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		input.CategoryID = &categoryID
	}

	product, err := h.productUC.CreateProduct(ctx, input)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.CreateProductResponse{
		Product: toProtoProduct(product),
	}), nil
}

func (h *ProductHandler) GetProduct(
	ctx context.Context,
	req *connect.Request[productv1.GetProductRequest],
) (*connect.Response[productv1.GetProductResponse], error) {
	productID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	product, err := h.productUC.GetProductWithSKUs(ctx, productID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.GetProductResponse{
		Product: toProtoProductWithSKUs(product),
	}), nil
}

func (h *ProductHandler) UpdateProduct(
	ctx context.Context,
	req *connect.Request[productv1.UpdateProductRequest],
) (*connect.Response[productv1.UpdateProductResponse], error) {
	productID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	input := usecase.UpdateProductInput{}
	if req.Msg.Name != nil {
		input.Name = req.Msg.Name
	}
	if req.Msg.Description != nil {
		input.Description = req.Msg.Description
	}
	if req.Msg.CategoryId != nil {
		categoryID, err := uuid.Parse(*req.Msg.CategoryId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		input.CategoryID = &categoryID
	}

	product, err := h.productUC.UpdateProduct(ctx, productID, input)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.UpdateProductResponse{
		Product: toProtoProduct(product),
	}), nil
}

func (h *ProductHandler) DeleteProduct(
	ctx context.Context,
	req *connect.Request[productv1.DeleteProductRequest],
) (*connect.Response[productv1.DeleteProductResponse], error) {
	productID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = h.productUC.DeleteProduct(ctx, productID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.DeleteProductResponse{}), nil
}

func (h *ProductHandler) ListProducts(
	ctx context.Context,
	req *connect.Request[productv1.ListProductsRequest],
) (*connect.Response[productv1.ListProductsResponse], error) {
	filter := domain.ProductFilter{}
	if req.Msg.CategoryId != nil {
		categoryID, err := uuid.Parse(*req.Msg.CategoryId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		filter.CategoryID = &categoryID
	}
	if req.Msg.SearchQuery != nil {
		filter.Search = req.Msg.SearchQuery
	}
	if req.Msg.Status != nil {
		status := toDomainProductStatus(*req.Msg.Status)
		filter.Status = &status
	}

	pageSize := req.Msg.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	pagination := domain.Pagination{
		PageSize:  pageSize,
		PageToken: req.Msg.PageToken,
	}

	products, total, err := h.productUC.ListProducts(ctx, filter, pagination)
	if err != nil {
		return nil, toConnectError(err)
	}

	resp := &productv1.ListProductsResponse{
		TotalCount: int32(total),
	}
	for _, p := range products {
		resp.Products = append(resp.Products, toProtoProduct(p))
	}

	return connect.NewResponse(resp), nil
}

func (h *ProductHandler) PublishProduct(
	ctx context.Context,
	req *connect.Request[productv1.PublishProductRequest],
) (*connect.Response[productv1.PublishProductResponse], error) {
	productID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = h.productUC.UpdateProductStatus(ctx, productID, domain.ProductStatusPublished)
	if err != nil {
		return nil, toConnectError(err)
	}

	product, err := h.productUC.GetProduct(ctx, productID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.PublishProductResponse{
		Product: toProtoProduct(product),
	}), nil
}

func (h *ProductHandler) HideProduct(
	ctx context.Context,
	req *connect.Request[productv1.HideProductRequest],
) (*connect.Response[productv1.HideProductResponse], error) {
	productID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = h.productUC.UpdateProductStatus(ctx, productID, domain.ProductStatusHidden)
	if err != nil {
		return nil, toConnectError(err)
	}

	product, err := h.productUC.GetProduct(ctx, productID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.HideProductResponse{
		Product: toProtoProduct(product),
	}), nil
}

func (h *ProductHandler) UnpublishProduct(
	ctx context.Context,
	req *connect.Request[productv1.UnpublishProductRequest],
) (*connect.Response[productv1.UnpublishProductResponse], error) {
	productID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = h.productUC.UpdateProductStatus(ctx, productID, domain.ProductStatusDraft)
	if err != nil {
		return nil, toConnectError(err)
	}

	product, err := h.productUC.GetProduct(ctx, productID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.UnpublishProductResponse{
		Product: toProtoProduct(product),
	}), nil
}

func (h *ProductHandler) CreateSKU(
	ctx context.Context,
	req *connect.Request[productv1.CreateSKURequest],
) (*connect.Response[productv1.CreateSKUResponse], error) {
	productID, err := uuid.Parse(req.Msg.ProductId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	input := usecase.CreateSKUInput{
		ProductID:       productID,
		SKUCode:         req.Msg.SkuCode,
		PriceAmount:     req.Msg.Price.Amount,
		PriceCurrency:   req.Msg.Price.CurrencyCode,
		Attributes:      req.Msg.Attributes,
		InitialQuantity: req.Msg.InitialQuantity,
	}

	sku, err := h.skuUC.CreateSKU(ctx, input)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.CreateSKUResponse{
		Sku: toProtoSKU(sku),
	}), nil
}

func (h *ProductHandler) GetSKU(
	ctx context.Context,
	req *connect.Request[productv1.GetSKURequest],
) (*connect.Response[productv1.GetSKUResponse], error) {
	skuID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	sku, err := h.skuUC.GetSKUWithInventory(ctx, skuID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.GetSKUResponse{
		Sku: toProtoSKUWithInventory(sku),
	}), nil
}

func (h *ProductHandler) UpdateSKU(
	ctx context.Context,
	req *connect.Request[productv1.UpdateSKURequest],
) (*connect.Response[productv1.UpdateSKUResponse], error) {
	skuID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	input := usecase.UpdateSKUInput{
		Attributes: req.Msg.Attributes,
	}
	if req.Msg.SkuCode != nil {
		input.SKUCode = req.Msg.SkuCode
	}
	if req.Msg.Price != nil {
		input.PriceAmount = &req.Msg.Price.Amount
		input.PriceCurrency = &req.Msg.Price.CurrencyCode
	}

	sku, err := h.skuUC.UpdateSKU(ctx, skuID, input)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.UpdateSKUResponse{
		Sku: toProtoSKU(sku),
	}), nil
}

func (h *ProductHandler) DeleteSKU(
	ctx context.Context,
	req *connect.Request[productv1.DeleteSKURequest],
) (*connect.Response[productv1.DeleteSKUResponse], error) {
	skuID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = h.skuUC.DeleteSKU(ctx, skuID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.DeleteSKUResponse{}), nil
}

func (h *ProductHandler) CreateCategory(
	ctx context.Context,
	req *connect.Request[productv1.CreateCategoryRequest],
) (*connect.Response[productv1.CreateCategoryResponse], error) {
	input := usecase.CreateCategoryInput{
		Name: req.Msg.Name,
	}
	if req.Msg.ParentId != nil {
		parentID, err := uuid.Parse(*req.Msg.ParentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		input.ParentID = &parentID
	}

	category, err := h.categoryUC.CreateCategory(ctx, input)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.CreateCategoryResponse{
		Category: toProtoCategory(category),
	}), nil
}

func (h *ProductHandler) GetCategory(
	ctx context.Context,
	req *connect.Request[productv1.GetCategoryRequest],
) (*connect.Response[productv1.GetCategoryResponse], error) {
	categoryID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	category, err := h.categoryUC.GetCategory(ctx, categoryID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.GetCategoryResponse{
		Category: toProtoCategory(category),
	}), nil
}

func (h *ProductHandler) ListCategories(
	ctx context.Context,
	req *connect.Request[productv1.ListCategoriesRequest],
) (*connect.Response[productv1.ListCategoriesResponse], error) {
	categories, err := h.categoryUC.ListCategories(ctx, nil)
	if err != nil {
		return nil, toConnectError(err)
	}

	resp := &productv1.ListCategoriesResponse{}
	for _, c := range categories {
		resp.Categories = append(resp.Categories, toProtoCategory(c))
	}

	return connect.NewResponse(resp), nil
}

func (h *ProductHandler) UpdateCategory(
	ctx context.Context,
	req *connect.Request[productv1.UpdateCategoryRequest],
) (*connect.Response[productv1.UpdateCategoryResponse], error) {
	categoryID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	input := usecase.UpdateCategoryInput{}
	if req.Msg.Name != nil {
		input.Name = req.Msg.Name
	}
	if req.Msg.ParentId != nil {
		parentID, err := uuid.Parse(*req.Msg.ParentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		input.ParentID = &parentID
	}

	category, err := h.categoryUC.UpdateCategory(ctx, categoryID, input)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.UpdateCategoryResponse{
		Category: toProtoCategory(category),
	}), nil
}

func (h *ProductHandler) DeleteCategory(
	ctx context.Context,
	req *connect.Request[productv1.DeleteCategoryRequest],
) (*connect.Response[productv1.DeleteCategoryResponse], error) {
	categoryID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = h.categoryUC.DeleteCategory(ctx, categoryID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.DeleteCategoryResponse{}), nil
}

func toDomainProductStatus(s productv1.ProductStatus) domain.ProductStatus {
	switch s {
	case productv1.ProductStatus_PRODUCT_STATUS_DRAFT:
		return domain.ProductStatusDraft
	case productv1.ProductStatus_PRODUCT_STATUS_PUBLISHED:
		return domain.ProductStatusPublished
	case productv1.ProductStatus_PRODUCT_STATUS_HIDDEN:
		return domain.ProductStatusHidden
	default:
		return domain.ProductStatusDraft
	}
}
