const settings = {
  storageConfig: {
    title: 'Storage Config',
    testUpload: 'Test Upload',
    provider: {
      aliyun: 'Aliyun OSS',
      tencent: 'Tencent COS',
      huawei: 'Huawei OBS',
      qiniu: 'Qiniu',
      minio: 'MinIO',
      aws: 'AWS S3',
      cloudflare: 'Cloudflare R2',
      custom: 'Custom'
    }
  },
  storageTest: {
    title: 'Storage Connectivity Test',
    selectConfig: 'Select Storage Config',
    selectFile: 'Select Test File',
    getCredential: 'Getting upload credentials...',
    uploading: 'Uploading to object storage...',
    verifySuccess: 'Configuration verified!',
    verifySuccessTips: 'AK/SK/Bucket/Permissions are all normal, record created',
    verifyFailed: 'Configuration verification failed',
    result: 'Test Result',
    getCredentialFailed: 'Failed to get upload credentials',
    close: 'Close',
    startTest: 'Start Test',
    form: {
      selectConfig: 'Please select storage config',
      selectFile: 'Please select file to upload',
      fileReading: 'File reading, please retry'
    }
  }
};

export default settings;
