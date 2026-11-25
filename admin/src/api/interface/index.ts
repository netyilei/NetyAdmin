// 请求响应参数（不包含data）
export interface Result {
  code: string;
  msg: string;
}

// 请求响应参数（包含data）
export interface ResultData<T = any> extends Result {
  data: T;
}

// V2 分页结构（api-style §3 契约：Page{list,total,page,page_size}；
// http.page<T> 以行类型为 T，返回 ResultData<Page<T>>）
export interface Page<T> {
  list: T[];
  total: number;
  page: number;
  page_size: number;
}

// 分页响应参数
export interface ResPage<T> {
  list: T[];
  pageNum: number;
  pageSize: number;
  total: number;
}

// 分页请求参数
export interface ReqPage {
  pageNum: number;
  pageSize: number;
}

// 文件上传模块
export namespace Upload {
  export interface ResFileUrl {
    fileUrl: string;
  }
}

// 登录模块
export namespace Login {
  export interface ReqLoginForm {
    username: string;
    password: string;
    captchaId: string;
    captchaCode: string;
  }
  export interface ResLogin {
    // V2 /admin/v1/auth/login（identity——信封 data）
    access_token: string;
    refresh_token: string;
    expires_in: number;
    must_change_password: boolean;
    profile: { id: string; username: string; name: string; email?: string };
  }
  export interface ResAuthButtons {
    // V2 /admin/v1/permissions——扁平权限码集（admin.*；v-auth 指令消费）
    acts: string[];
  }
}

// 用户管理模块
export namespace User {
  export interface ReqUserParams extends ReqPage {
    username: string;
    gender: number;
    idCard: string;
    email: string;
    address: string;
    createTime: string[];
    status: number;
  }
  export interface ResUserList {
    id: string;
    username: string;
    gender: number;
    user: { detail: { age: number } };
    idCard: string;
    email: string;
    address: string;
    createTime: string;
    status: number;
    avatar: string;
    photo: any[];
    children?: ResUserList[];
  }
  export interface ResStatus {
    userLabel: string;
    userValue: number;
  }
  export interface ResGender {
    genderLabel: string;
    genderValue: number;
  }
  export interface ResDepartment {
    id: string;
    name: string;
    children?: ResDepartment[];
  }
  export interface ResRole {
    id: string;
    name: string;
    children?: ResDepartment[];
  }
}
