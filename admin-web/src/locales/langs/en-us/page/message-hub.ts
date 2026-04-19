const messageHub = {
  template: {
    title: 'Message Template',
    code: 'Template Code',
    name: 'Template Name',
    channel: 'Channel',
    msgTitle: 'Message Title',
    content: 'Template Content',
    providerTplId: 'Provider Template ID',
    status: 'Status',
    time: 'Created At',
    test: 'Test Send',
    form: {
      codePlaceholder: 'Please enter template code',
      namePlaceholder: 'Please enter template name',
      channelPlaceholder: 'Please select channel',
      titlePlaceholder: 'Please enter message title',
      contentPlaceholder: 'Please enter template content',
      providerTplIdPlaceholder: 'Please enter provider template ID'
    }
  },
  record: {
    title: 'Send Records',
    receiver: 'Receiver',
    channel: 'Channel',
    status: 'Status',
    errorMsg: 'Error Message',
    time: 'Sent At',
    priority: 'Priority',
    retryCount: 'Retry Count',
    detail: 'Detail',
    resend: 'Resend',
    retry: 'Retry',
    sendSuccess: 'Sent',
    sendFailed: 'Failed',
    pending: 'Pending'
  },
  channel: {
    sms: 'SMS',
    email: 'Email',
    internal: 'Internal',
    push: 'Push'
  },
  priority: {
    high: 'High',
    medium: 'Medium',
    low: 'Low'
  },
  send: {
    contentMode: 'Content Mode',
    customContent: 'Custom Content',
    templateContent: 'Template',
    selectTemplate: 'Select template',
    phonePlaceholder: 'Enter phone or search user',
    phoneHint: 'Type a phone number and press Enter, or select from user list',
    emailPlaceholder: 'Enter email or search user',
    emailHint: 'Type an email address and press Enter, or select from user list',
    customSmsPlaceholder: 'Enter SMS content',
    emailTitlePlaceholder: 'Enter email subject',
    templateAutoFill: 'Template content auto-filled'
  }
};

export default messageHub;
