import axios from 'axios';

// آدرس سرور بک‌اند (IP سرور خودت رو جایگزین localhost کن اگر روی سرور واقعی هستی)
// نکته: چون این کد توی مرورگر کاربر اجرا میشه، باید IP عمومی سرور رو بدی
const API_BASE_URL = 'http://YOUR_SERVER_IP:8080/api';

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// اضافه کردن یک تاخیر مصنوعی برای تست لودینگ (اختیاری - بعدا پاکش کن)
// apiClient.interceptors.response.use(async (response) => {
//   await new Promise(resolve => setTimeout(resolve, 500));
//   return response;
// });