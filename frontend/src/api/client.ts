import axios from 'axios';

// آدرس سرور: همیشه از مسیر نسبی استفاده می‌کنیم تا با IP و دامنه کار کند
// این باعث می‌شود که درخواست‌ها همیشه به همان host (IP یا دامنه) که از آن وارد شده‌ایم ارسال شوند
const getApiBaseUrl = () => {
  // در browser از window.location استفاده می‌کنیم
  if (typeof window !== 'undefined') {
    return `${window.location.origin}/api`;
  }
  // در server-side یا development از متغیر محیطی استفاده می‌کنیم
  return import.meta.env.VITE_API_URL || 'http://localhost:8080/api';
};

const API_BASE_URL = getApiBaseUrl();

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// 👇👇👇 این بخش جدید است: اینترسپتور برای اضافه کردن توکن
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// (اختیاری) هندل کردن خطای 401 برای خروج خودکار
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // فقط در صورتی توکن را حذف می‌کنیم که واقعاً مشکل از توکن باشد.
      // برای خطاهای منطقی مثل "Current password is incorrect" نباید کاربر از سیستم خارج شود.
      const apiErrorMsg = (error.response?.data?.error || '').toString();
      const shouldLogout =
        apiErrorMsg === 'Invalid or expired token' ||
        apiErrorMsg === 'Missing authorization token' ||
        apiErrorMsg === 'Invalid token format';

      if (shouldLogout) {
      localStorage.removeItem('token');
        localStorage.removeItem('username');
        localStorage.removeItem('role');
      // window.location.href = '/login'; // فعلا کامنت می‌کنیم تا ریدایرکت دستی انجام بشه
      }
    }
    return Promise.reject(error);
  }
);