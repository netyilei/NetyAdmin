export namespace Content {
  type ContentType = 'plaintext' | 'richtext';

  type PublishStatus = 'draft' | 'published' | 'scheduled';

  type LinkType = 'none' | 'internal' | 'external' | 'article';

  type Category = {
    id: number;
    parentId: number;
    name: string;
    code: string;
    icon: string;
    sort: number;
    storageConfigId: number | null;
    contentType: ContentType;
    status: import('@/typings/api/v1/common').Common.EnableStatus;
    remark: string;
    createdBy: number;
    updatedBy: number;
    createdAt: string;
    updatedAt: string;
  };

  type CategoryTree = Category & {
    children: CategoryTree[];
  };

  type CategorySearchParams = import('@/typings/api/v1/common').Common.CommonSearchParams & {
    name?: string;
    code?: string;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
    startTime?: string;
    endTime?: string;
  };

  type CategoryList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Category>;

  type CreateCategoryParams = {
    parentId?: number;
    name: string;
    code?: string;
    icon?: string;
    sort?: number;
    storageConfigId?: number | null;
    contentType?: ContentType;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
    remark?: string;
  };

  type UpdateCategoryParams = {
    parentId?: number;
    name?: string;
    code?: string;
    icon?: string;
    sort?: number;
    storageConfigId?: number | null;
    contentType?: ContentType;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
    remark?: string;
  };

  type Article = {
    id: number;
    categoryId: number;
    categoryName: string;
    title: string;
    titleColor: string;
    coverImage: string;
    summary: string;
    content: string;
    contentType: ContentType;
    author: string;
    source: string;
    keywords: string;
    tags: string;
    isTop: boolean;
    topSort: number;
    isHot: boolean;
    isRecommend: boolean;
    allowComment: boolean;
    viewCount: number;
    likeCount: number;
    commentCount: number;
    publishStatus: PublishStatus;
    publishedAt: string | null;
    scheduledAt: string | null;
    status: import('@/typings/api/v1/common').Common.EnableStatus;
    createdBy: number;
    updatedBy: number;
    createdAt: string;
    updatedAt: string;
    category?: Category;
  };

  type ArticleSearchParams = import('@/typings/api/v1/common').Common.CommonSearchParams & {
    categoryId?: number;
    title?: string;
    author?: string;
    publishStatus?: PublishStatus;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
    isTop?: string;
    startTime?: string;
    endTime?: string;
  };

  type ArticleList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Article>;

  type CreateArticleParams = {
    categoryId: number;
    title: string;
    titleColor?: string;
    coverImage?: string;
    summary?: string;
    content?: string;
    contentType?: ContentType;
    author?: string;
    source?: string;
    keywords?: string;
    tags?: string;
    isTop?: boolean;
    topSort?: number;
    isHot?: boolean;
    isRecommend?: boolean;
    allowComment?: boolean;
    publishStatus?: PublishStatus;
    scheduledAt?: number | null;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
  };

  type UpdateArticleParams = {
    categoryId?: number;
    title?: string;
    titleColor?: string;
    coverImage?: string;
    summary?: string;
    content?: string;
    contentType?: ContentType;
    author?: string;
    source?: string;
    keywords?: string;
    tags?: string;
    isTop?: boolean;
    topSort?: number;
    isHot?: boolean;
    isRecommend?: boolean;
    allowComment?: boolean;
    publishStatus?: PublishStatus;
    scheduledAt?: number | null;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
  };

  type SetArticleTopParams = {
    isTop: boolean;
    topSort?: number;
  };

  type BannerGroup = {
    id: number;
    name: string;
    code: string;
    description: string;
    position: string;
    width: number;
    height: number;
    maxItems: number;
    autoPlay: boolean;
    interval: number;
    sort: number;
    storageConfigId: number | null;
    status: import('@/typings/api/v1/common').Common.EnableStatus;
    remark: string;
    createdBy: number;
    updatedBy: number;
    createdAt: string;
    updatedAt: string;
  };

  type BannerGroupSearchParams = import('@/typings/api/v1/common').Common.CommonSearchParams & {
    name?: string;
    code?: string;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
    startTime?: string;
    endTime?: string;
  };

  type BannerGroupList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<BannerGroup>;

  type CreateBannerGroupParams = {
    name: string;
    code: string;
    description?: string;
    position?: string;
    width?: number;
    height?: number;
    maxItems?: number;
    storageConfigId?: number | null;
    autoPlay?: boolean;
    interval?: number;
    sort?: number;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
    remark?: string;
  };

  type UpdateBannerGroupParams = {
    name?: string;
    code?: string;
    description?: string;
    position?: string;
    width?: number;
    height?: number;
    maxItems?: number;
    storageConfigId?: number | null;
    autoPlay?: boolean;
    interval?: number;
    sort?: number;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
    remark?: string;
  };

  type Banner = {
    id: number;
    groupId: number;
    title: string;
    subtitle: string;
    imageUrl: string;
    imageAlt: string;
    linkType: LinkType;
    linkUrl: string;
    linkArticleId: number | null;
    content: string;
    customParams: string;
    sort: number;
    startTime: string | null;
    endTime: string | null;
    viewCount: number;
    clickCount: number;
    status: import('@/typings/api/v1/common').Common.EnableStatus;
    createdBy: number;
    updatedBy: number;
    createdAt: string;
    updatedAt: string;
  };

  type BannerSearchParams = import('@/typings/api/v1/common').Common.CommonSearchParams & {
    groupId?: number;
    title?: string;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
  };

  type BannerList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Banner>;

  type CreateBannerParams = {
    groupId: number;
    title: string;
    subtitle?: string;
    imageUrl: string;
    imageAlt?: string;
    linkType?: LinkType;
    linkUrl?: string;
    linkArticleId?: number;
    content?: string;
    customParams?: string;
    sort?: number;
    startTime?: number | null;
    endTime?: number | null;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
  };

  type UpdateBannerParams = {
    title?: string;
    subtitle?: string;
    imageUrl?: string;
    imageAlt?: string;
    linkType?: LinkType;
    linkUrl?: string;
    linkArticleId?: number;
    content?: string;
    customParams?: string;
    sort?: number;
    startTime?: number | null;
    endTime?: number | null;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
  };

  type BannerGroupWithBanners = BannerGroup & {
    banners: Banner[];
  };
}
