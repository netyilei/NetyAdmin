const settings = {
  storageConfig: {
    title: '存储配置',
    testUpload: '测试上传',
    provider: {
      aliyun: '阿里云 OSS',
      tencent: '腾讯云 COS',
      huawei: '华为云 OBS',
      qiniu: '七牛云',
      minio: 'MinIO',
      aws: 'AWS S3',
      cloudflare: 'Cloudflare R2',
      custom: '自定义'
    }
  },
  storageTest: {
    title: '存储配置连通性测试',
    selectConfig: '选择存储配置',
    selectFile: '选择测试文件',
    getCredential: '正在获取上传凭证...',
    uploading: '正在直传至对象存储...',
    verifySuccess: '配置验证通过！',
    verifySuccessTips: 'AK/SK/Bucket/权限全部正常，记录已入库',
    verifyFailed: '配置验证失败',
    result: '测试结果',
    getCredentialFailed: '获取上传凭证失败',
    close: '关闭',
    startTest: '开始测试',
    form: {
      selectConfig: '请选择存储配置',
      selectFile: '请选择要上传的文件',
      fileReading: '文件读取中，请重试'
    }
  }
};

export default settings;
