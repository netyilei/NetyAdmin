import { APP_CONFIG } from '@/config';

const system = {
  title: APP_CONFIG.name,
  updateTitle: 'System Version Update Notification',
  updateContent: 'A new version of the system has been detected. Do you want to refresh the page immediately?',
  updateConfirm: 'Refresh immediately',
  updateCancel: 'Later'
};

export default system;
