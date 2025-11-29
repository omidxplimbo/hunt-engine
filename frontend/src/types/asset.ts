export interface Asset {
  id: number;
  value: string;
  type: string;
  is_new: boolean;
  is_live: boolean;
  created_at: string;
  
  // فیلدهای فاز ۲ (پروبینگ)
  final_url?: string;
  status_code?: number;
  title?: string;
  content_length?: number;
  host_ip?: string; // نکته: این فیلد فعلا به صورت رشته JSON میاد
  dnsx_ip?: string; // 👈 فیلد جدید
  web_server?: string;
  cdn_name?: string;
  technologies?: string[]; // آرایه‌ای از نام تکنولوژی‌ها
  response_time_ms?: number;
  
  // داده خام (اختیاری)
  raw_httpx?: any;
}

export interface AssetResponse {
  status: string;
  data: Asset[];
  page_count: number;
  total_count: number;
}


export interface AssetFilters {
  is_live?: boolean;
  is_new?: boolean;
  search?: string; // 👈 فیلد جدید
}