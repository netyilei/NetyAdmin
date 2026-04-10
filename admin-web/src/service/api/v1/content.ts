import type { Content } from '@/typings/api/v1/content';
import { request } from '../../request';

/**
 * Content Article
 */

export function fetchGetArticleList(params?: Content.ArticleSearchParams) {
  return request<Content.ArticleList>({
    url: '/admin/v1/content/articles',
    method: 'get',
    params
  });
}

export function fetchGetArticle(id: number) {
  return request<Content.Article>({
    url: `/admin/v1/content/articles/${id}`,
    method: 'get'
  });
}

export function fetchCreateArticle(data: Content.CreateArticleParams) {
  return request<Content.Article>({
    url: '/admin/v1/content/articles',
    method: 'post',
    data
  });
}

export function fetchUpdateArticle(id: number, data: Content.UpdateArticleParams) {
  return request<Content.Article>({
    url: `/admin/v1/content/articles/${id}`,
    method: 'put',
    data
  });
}

export function fetchDeleteArticle(id: number) {
  return request({
    url: `/admin/v1/content/articles/${id}`,
    method: 'delete'
  });
}

export function fetchPublishArticle(id: number) {
  return request({
    url: `/admin/v1/content/articles/${id}/publish`,
    method: 'put'
  });
}

export function fetchUnpublishArticle(id: number) {
  return request({
    url: `/admin/v1/content/articles/${id}/unpublish`,
    method: 'put'
  });
}

export function fetchSetArticleTop(id: number, data: Content.SetArticleTopParams) {
  return request({
    url: `/admin/v1/content/articles/${id}/top`,
    method: 'put',
    data
  });
}

/**
 * Content Banner Group
 */

export function fetchGetBannerGroupList(params?: Content.BannerGroupSearchParams) {
  return request<Content.BannerGroupList>({
    url: '/admin/v1/content/banner-groups',
    method: 'get',
    params
  });
}

export function fetchGetBannerGroup(id: number) {
  return request<Content.BannerGroup>({
    url: `/admin/v1/content/banner-groups/${id}`,
    method: 'get'
  });
}

export function fetchGetAllBannerGroups() {
  return request<Content.BannerGroup[]>({
    url: '/admin/v1/content/banner-groups',
    method: 'get',
    params: { size: 100 }
  });
}

export function fetchCreateBannerGroup(data: Content.CreateBannerGroupParams) {
  return request<Content.BannerGroup>({
    url: '/admin/v1/content/banner-groups',
    method: 'post',
    data
  });
}

export function fetchUpdateBannerGroup(id: number, data: Content.UpdateBannerGroupParams) {
  return request<Content.BannerGroup>({
    url: `/admin/v1/content/banner-groups/${id}`,
    method: 'put',
    data
  });
}

export function fetchDeleteBannerGroup(id: number) {
  return request({
    url: `/admin/v1/content/banner-groups/${id}`,
    method: 'delete'
  });
}

/**
 * Content Banner Item
 */

export function fetchGetBannerItemList(params?: Content.BannerItemSearchParams) {
  return request<Content.BannerItemList>({
    url: '/admin/v1/content/banner-items',
    method: 'get',
    params
  });
}

export function fetchGetBannerItem(id: number) {
  return request<Content.BannerItem>({
    url: `/admin/v1/content/banner-items/${id}`,
    method: 'get'
  });
}

export function fetchCreateBannerItem(data: Content.CreateBannerItemParams) {
  return request<Content.BannerItem>({
    url: '/admin/v1/content/banner-items',
    method: 'post',
    data
  });
}

export function fetchUpdateBannerItem(id: number, data: Content.UpdateBannerItemParams) {
  return request<Content.BannerItem>({
    url: `/admin/v1/content/banner-items/${id}`,
    method: 'put',
    data
  });
}

export function fetchDeleteBannerItem(id: number) {
  return request({
    url: `/admin/v1/content/banner-items/${id}`,
    method: 'delete'
  });
}

/**
 * Content Category
 */

export function fetchGetCategoryList(params?: Content.CategorySearchParams) {
  return request<Content.CategoryList>({
    url: '/admin/v1/content/categories',
    method: 'get',
    params
  });
}

export function fetchGetCategoryTree(refresh = false) {
  return request<Content.CategoryTree[]>({
    url: '/admin/v1/content/categories/tree',
    method: 'get',
    params: { refresh }
  });
}

export function fetchGetCategory(id: number) {
  return request<Content.Category>({
    url: `/admin/v1/content/categories/${id}`,
    method: 'get'
  });
}

export function fetchCreateCategory(data: Content.CreateCategoryParams) {
  return request<Content.Category>({
    url: '/admin/v1/content/categories',
    method: 'post',
    data
  });
}

export function fetchUpdateCategory(id: number, data: Content.UpdateCategoryParams) {
  return request<Content.Category>({
    url: `/admin/v1/content/categories/${id}`,
    method: 'put',
    data
  });
}

export function fetchDeleteCategory(id: number) {
  return request({
    url: `/admin/v1/content/categories/${id}`,
    method: 'delete'
  });
}
