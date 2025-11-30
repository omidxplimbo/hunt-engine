import axios from 'axios';

// آدرس سرور از متغیر محیطی یا پیش‌فرض
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api';

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
      // اگر توکن منقضی شده بود، کاربر رو بنداز بیرون
      localStorage.removeItem('token');
      // window.location.href = '/login'; // فعلا کامنت می‌کنیم تا ریدایرکت دستی انجام بشه
    }
    return Promise.reject(error);
  }
);