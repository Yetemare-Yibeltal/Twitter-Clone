// frontend/src/services/api.ts
import axios, {
  AxiosInstance,
  AxiosRequestConfig,
  AxiosResponse,
  AxiosError,
  InternalAxiosRequestConfig,
} from 'axios';
import { API_CONFIG, API_ENDPOINTS } from '../config/api.config';
import { getFromStorage, setToStorage, removeFromStorage, StorageKeys } from '../utils/storage';

// ======================================================================
// Types
// ======================================================================

export interface ApiResponse<T = any> {
  data: T;
  message?: string;
  status: number;
  success: boolean;
}

export interface ApiError {
  code: string;
  message: string;
  status: number;
  details?: Record<string, any>;
}

export interface AuthTokens {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
  tokenType: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
  nextCursor?: string;
}

// ======================================================================
= API Client Class
// ======================================================================

class ApiClient {
  private instance: AxiosInstance;
  private isRefreshing: boolean = false;
  private refreshSubscribers: Array<(token: string) => void> = [];
  private baseURL: string;

  constructor() {
    this.baseURL = API_CONFIG.BASE_URL;
    this.instance = axios.create({
      baseURL: this.baseURL,
      timeout: API_CONFIG.TIMEOUT,
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
    });
    this.setupInterceptors();
  }

  // ======================================================================
  // Interceptors
  // ======================================================================

  private setupInterceptors(): void {
    // Request interceptor
    this.instance.interceptors.request.use(
      this.handleRequest.bind(this),
      this.handleRequestError.bind(this)
    );

    // Response interceptor
    this.instance.interceptors.response.use(
      this.handleResponse.bind(this),
      this.handleResponseError.bind(this)
    );
  }

  private handleRequest(config: InternalAxiosRequestConfig): InternalAxiosRequestConfig {
    // Add token if available
    const token = this.getAccessToken();
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }

    // Add request ID for tracing
    const requestId = this.generateRequestId();
    config.headers['X-Request-ID'] = requestId;

    // Log request in development
    if (API_CONFIG.DEBUG) {
      console.log(`[API] ${config.method?.toUpperCase()} ${config.url}`, {
        data: config.data,
        params: config.params,
        headers: config.headers,
      });
    }

    return config;
  }

  private handleRequestError(error: AxiosError): Promise<AxiosError> {
    if (API_CONFIG.DEBUG) {
      console.error('[API] Request error:', error);
    }
    return Promise.reject(error);
  }

  private handleResponse(response: AxiosResponse): AxiosResponse {
    if (API_CONFIG.DEBUG) {
      console.log(`[API] Response ${response.status} ${response.config.url}`, {
        data: response.data,
      });
    }
    return response;
  }

  private handleResponseError(error: AxiosError): Promise<any> {
    const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean };

    if (API_CONFIG.DEBUG) {
      console.error('[API] Response error:', {
        status: error.response?.status,
        data: error.response?.data,
        url: error.config?.url,
        method: error.config?.method,
      });
    }

    // Handle 401 Unauthorized - token expired
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;
      return this.handleTokenRefresh(originalRequest);
    }

    // Handle 429 Rate Limit
    if (error.response?.status === 429) {
      const retryAfter = parseInt(error.response.headers['retry-after'] || '5', 10);
      return this.handleRateLimit(retryAfter, originalRequest);
    }

    // Handle 503 Service Unavailable - retry with backoff
    if (error.response?.status === 503) {
      const retryCount = (originalRequest as any).retryCount || 0;
      if (retryCount < 3) {
        (originalRequest as any).retryCount = retryCount + 1;
        const delay = Math.pow(2, retryCount) * 1000;
        return this.handleRetry(delay, originalRequest);
      }
    }

    return Promise.reject(this.normalizeError(error));
  }

  // ======================================================================
  // Token Refresh
  // ======================================================================

  private handleTokenRefresh(originalRequest: InternalAxiosRequestConfig): Promise<any> {
    return new Promise((resolve, reject) => {
      // Subscribe to refresh event
      this.subscribeRefresh((token: string) => {
        originalRequest.headers.Authorization = `Bearer ${token}`;
        resolve(this.instance.request(originalRequest));
      });

      // If not already refreshing, start refresh process
      if (!this.isRefreshing) {
        this.isRefreshing = true;
        this.refreshTokens()
          .then((newTokens) => {
            this.isRefreshing = false;
            this.onRefreshSuccess(newTokens);
          })
          .catch((error) => {
            this.isRefreshing = false;
            this.onRefreshFailure(error);
            reject(error);
          });
      }
    });
  }

  private subscribeRefresh(callback: (token: string) => void): void {
    this.refreshSubscribers.push(callback);
  }

  private onRefreshSuccess(tokens: AuthTokens): void {
    this.setTokens(tokens);
    this.refreshSubscribers.forEach((callback) => callback(tokens.accessToken));
    this.refreshSubscribers = [];
  }

  private onRefreshFailure(error: any): void {
    this.clearTokens();
    this.refreshSubscribers = [];
    window.dispatchEvent(new CustomEvent('auth:logout'));
  }

  private async refreshTokens(): Promise<AuthTokens> {
    const refreshToken = this.getRefreshToken();
    if (!refreshToken) {
      throw new Error('No refresh token available');
    }

    const response = await this.instance.post('/api/auth/refresh', {
      refresh_token: refreshToken,
    });

    if (response.status !== 200) {
      throw new Error('Token refresh failed');
    }

    return {
      accessToken: response.data.access_token,
      refreshToken: response.data.refresh_token,
      expiresIn: response.data.expires_in,
      tokenType: response.data.token_type || 'Bearer',
    };
  }

  // ======================================================================
  // Rate Limit and Retry
  // ======================================================================

  private handleRateLimit(retryAfter: number, originalRequest: InternalAxiosRequestConfig): Promise<any> {
    return new Promise((resolve) => {
      setTimeout(() => {
        resolve(this.instance.request(originalRequest));
      }, retryAfter * 1000);
    });
  }

  private handleRetry(delay: number, originalRequest: InternalAxiosRequestConfig): Promise<any> {
    return new Promise((resolve) => {
      setTimeout(() => {
        resolve(this.instance.request(originalRequest));
      }, delay);
    });
  }

  // ======================================================================
  // Error Normalization
  // ======================================================================

  private normalizeError(error: AxiosError): ApiError {
    if (error.response) {
      const data = error.response.data as any;
      return {
        code: data.code || `HTTP_${error.response.status}`,
        message: data.message || data.error || error.message || 'An error occurred',
        status: error.response.status,
        details: data.details || data.errors,
      };
    }
    if (error.request) {
      return {
        code: 'NETWORK_ERROR',
        message: 'Network error occurred. Please check your connection.',
        status: 0,
      };
    }
    return {
      code: 'UNKNOWN_ERROR',
      message: error.message || 'An unknown error occurred',
      status: 500,
    };
  }

  // ======================================================================
  // Token Management
  // ======================================================================

  private getAccessToken(): string | null {
    return getFromStorage<string>(StorageKeys.ACCESS_TOKEN);
  }

  private getRefreshToken(): string | null {
    return getFromStorage<string>(StorageKeys.REFRESH_TOKEN);
  }

  private setTokens(tokens: AuthTokens): void {
    setToStorage(StorageKeys.ACCESS_TOKEN, tokens.accessToken);
    setToStorage(StorageKeys.REFRESH_TOKEN, tokens.refreshToken);
  }

  private clearTokens(): void {
    removeFromStorage(StorageKeys.ACCESS_TOKEN);
    removeFromStorage(StorageKeys.REFRESH_TOKEN);
  }

  // ======================================================================
  // Request Methods
  // ======================================================================

  public async get<T = any>(
    url: string,
    config?: AxiosRequestConfig
  ): Promise<ApiResponse<T>> {
    const response = await this.instance.get<T>(url, config);
    return this.formatResponse(response);
  }

  public async post<T = any>(
    url: string,
    data?: any,
    config?: AxiosRequestConfig
  ): Promise<ApiResponse<T>> {
    const response = await this.instance.post<T>(url, data, config);
    return this.formatResponse(response);
  }

  public async put<T = any>(
    url: string,
    data?: any,
    config?: AxiosRequestConfig
  ): Promise<ApiResponse<T>> {
    const response = await this.instance.put<T>(url, data, config);
    return this.formatResponse(response);
  }

  public async patch<T = any>(
    url: string,
    data?: any,
    config?: AxiosRequestConfig
  ): Promise<ApiResponse<T>> {
    const response = await this.instance.patch<T>(url, data, config);
    return this.formatResponse(response);
  }

  public async delete<T = any>(
    url: string,
    config?: AxiosRequestConfig
  ): Promise<ApiResponse<T>> {
    const response = await this.instance.delete<T>(url, config);
    return this.formatResponse(response);
  }

  public async upload<T = any>(
    url: string,
    file: File,
    onProgress?: (percentage: number) => void,
    config?: AxiosRequestConfig
  ): Promise<ApiResponse<T>> {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('filename', file.name);

    const response = await this.instance.post<T>(url, formData, {
      ...config,
      headers: {
        'Content-Type': 'multipart/form-data',
        ...config?.headers,
      },
      onUploadProgress: (event) => {
        if (event.total && onProgress) {
          const percentage = Math.round((event.loaded * 100) / event.total);
          onProgress(percentage);
        }
      },
    });
    return this.formatResponse(response);
  }

  public async uploadMultiple<T = any>(
    url: string,
    files: File[],
    onProgress?: (fileIndex: number, percentage: number) => void,
    config?: AxiosRequestConfig
  ): Promise<ApiResponse<T>> {
    const formData = new FormData();
    files.forEach((file, index) => {
      formData.append(`files[${index}]`, file);
      formData.append(`filenames[${index}]`, file.name);
    });

    const response = await this.instance.post<T>(url, formData, {
      ...config,
      headers: {
        'Content-Type': 'multipart/form-data',
        ...config?.headers,
      },
      onUploadProgress: (event) => {
        if (event.total && onProgress) {
          // Simplified progress tracking
          const totalProgress = Math.round((event.loaded * 100) / event.total);
          onProgress(0, totalProgress);
        }
      },
    });
    return this.formatResponse(response);
  }

  private formatResponse<T>(response: AxiosResponse): ApiResponse<T> {
    return {
      data: response.data,
      status: response.status,
      success: response.status >= 200 && response.status < 300,
      message: response.data?.message || response.statusText,
    };
  }

  // ======================================================================
  // Helper Methods
  // ======================================================================

  private generateRequestId(): string {
    return `req_${Date.now()}_${Math.random().toString(36).substring(2, 9)}`;
  }

  public setBaseURL(url: string): void {
    this.baseURL = url;
    this.instance.defaults.baseURL = url;
  }

  public getBaseURL(): string {
    return this.baseURL;
  }

  public setDefaultHeader(key: string, value: string): void {
    this.instance.defaults.headers.common[key] = value;
  }

  public removeDefaultHeader(key: string): void {
    delete this.instance.defaults.headers.common[key];
  }

  // ======================================================================
  // API Endpoints
  // ======================================================================

  // ---------- Auth ----------
  public auth = {
    register: (data: any) => this.post<any>('/api/auth/register', data),
    login: (data: any) => this.post<any>('/api/auth/login', data),
    logout: () => this.post<any>('/api/auth/logout'),
    refresh: (data: any) => this.post<any>('/api/auth/refresh', data),
    verifyEmail: (token: string) => this.post<any>(`/api/auth/verify-email`, { token }),
    resendVerification: () => this.post<any>('/api/auth/verify-email/resend'),
    forgotPassword: (email: string) => this.post<any>('/api/auth/forgot-password', { email }),
    resetPassword: (data: any) => this.post<any>('/api/auth/reset-password', data),
    changePassword: (data: any) => this.post<any>('/api/auth/change-password', data),
    getSessions: () => this.get<any>('/api/auth/sessions'),
    revokeSession: (sessionId: string) => this.delete<any>(`/api/auth/sessions/${sessionId}`),
    revokeAllSessions: () => this.post<any>('/api/auth/sessions/revoke-all'),
  };

  // ---------- Users ----------
  public users = {
    getProfile: (identifier: string) => this.get<any>(`/api/users/${identifier}`),
    updateProfile: (data: any) => this.put<any>('/api/users/profile', data),
    follow: (userId: string) => this.post<any>(`/api/users/${userId}/follow`),
    unfollow: (userId: string) => this.post<any>(`/api/users/${userId}/unfollow`),
    getFollowers: (userId: string, params?: any) => this.get<any>(`/api/users/${userId}/followers`, { params }),
    getFollowing: (userId: string, params?: any) => this.get<any>(`/api/users/${userId}/following`, { params }),
    getMutualFollows: (userId: string, params?: any) => this.get<any>(`/api/users/${userId}/mutual`, { params }),
    checkFollowStatus: (userId: string) => this.get<any>(`/api/users/${userId}/follow-status`),
    getStats: (userId: string) => this.get<any>(`/api/users/${userId}/stats`),
    getTweets: (userId: string, params?: any) => this.get<any>(`/api/users/${userId}/tweets`, { params }),
    search: (params: any) => this.get<any>('/api/users/search', { params }),
    getSuggestions: (params?: any) => this.get<any>('/api/users/suggestions', { params }),
    getOnlineStatus: (userId: string) => this.get<any>(`/api/users/${userId}/online`),
  };

  // ---------- Tweets ----------
  public tweets = {
    create: (data: any) => this.post<any>('/api/tweets', data),
    get: (tweetId: string) => this.get<any>(`/api/tweets/${tweetId}`),
    update: (tweetId: string, data: any) => this.put<any>(`/api/tweets/${tweetId}`, data),
    delete: (tweetId: string) => this.delete<any>(`/api/tweets/${tweetId}`),
    getFeed: (params?: any) => this.get<any>('/api/tweets/feed', { params }),
    getReplies: (tweetId: string, params?: any) => this.get<any>(`/api/tweets/${tweetId}/replies`, { params }),
    like: (tweetId: string) => this.post<any>(`/api/tweets/${tweetId}/like`),
    unlike: (tweetId: string) => this.delete<any>(`/api/tweets/${tweetId}/like`),
    retweet: (tweetId: string) => this.post<any>(`/api/tweets/${tweetId}/retweet`),
    unretweet: (tweetId: string) => this.delete<any>(`/api/tweets/${tweetId}/retweet`),
    bookmark: (tweetId: string) => this.post<any>(`/api/tweets/${tweetId}/bookmark`),
    unbookmark: (tweetId: string) => this.delete<any>(`/api/tweets/${tweetId}/bookmark`),
    quote: (tweetId: string, data: any) => this.post<any>(`/api/tweets/${tweetId}/quote`, data),
    getBookmarks: (params?: any) => this.get<any>('/api/bookmarks', { params }),
    getTrending: (params?: any) => this.get<any>('/api/trending', { params }),
  };

  // ---------- Polls ----------
  public polls = {
    vote: (pollId: string, data: any) => this.post<any>(`/api/polls/${pollId}/vote`, data),
    getResults: (pollId: string) => this.get<any>(`/api/polls/${pollId}/results`),
  };

  // ---------- Communities ----------
  public communities = {
    create: (data: any) => this.post<any>('/api/communities', data),
    get: (id: string) => this.get<any>(`/api/communities/${id}`),
    getBySlug: (slug: string) => this.get<any>(`/api/communities/slug/${slug}`),
    update: (id: string, data: any) => this.put<any>(`/api/communities/${id}`, data),
    delete: (id: string) => this.delete<any>(`/api/communities/${id}`),
    list: (params?: any) => this.get<any>('/api/communities', { params }),
    search: (params: any) => this.get<any>('/api/communities/search', { params }),
    join: (id: string) => this.post<any>(`/api/communities/${id}/join`),
    leave: (id: string) => this.post<any>(`/api/communities/${id}/leave`),
    getMembers: (id: string, params?: any) => this.get<any>(`/api/communities/${id}/members`, { params }),
    getPosts: (id: string, params?: any) => this.get<any>(`/api/communities/${id}/posts`, { params }),
    addPost: (id: string, tweetId: string) => this.post<any>(`/api/communities/${id}/posts/${tweetId}`),
    removePost: (id: string, tweetId: string) => this.delete<any>(`/api/communities/${id}/posts/${tweetId}`),
    banUser: (id: string, userId: string, data: any) => this.post<any>(`/api/communities/${id}/ban/${userId}`, data),
    unbanUser: (id: string, userId: string) => this.delete<any>(`/api/communities/${id}/ban/${userId}`),
    updateMemberRole: (id: string, userId: string, data: any) => this.put<any>(`/api/communities/${id}/members/${userId}/role`, data),
    removeMember: (id: string, userId: string) => this.delete<any>(`/api/communities/${id}/members/${userId}`),
    getTrending: (params?: any) => this.get<any>('/api/communities/trending', { params }),
    getRecommendations: (params?: any) => this.get<any>('/api/communities/recommendations', { params }),
    getStats: () => this.get<any>('/api/admin/communities/stats'),
  };

  // ---------- Direct Messages ----------
  public dms = {
    send: (data: any) => this.post<any>('/api/dms/send', data),
    getConversation: (userId: string, params?: any) => this.get<any>(`/api/dms/conversation/${userId}`, { params }),
    getConversations: (params?: any) => this.get<any>('/api/dms/conversations', { params }),
    markAsRead: (data: any) => this.post<any>('/api/dms/mark-read', data),
    markConversationAsRead: (userId: string) => this.post<any>(`/api/dms/conversation/${userId}/read`),
    markAllAsRead: () => this.post<any>('/api/dms/mark-all-read'),
    delete: (data: any) => this.post<any>('/api/dms/delete', data),
    deleteConversation: (userId: string) => this.delete<any>(`/api/dms/conversation/${userId}`),
    search: (params: any) => this.get<any>('/api/dms/search', { params }),
    getUnreadCount: () => this.get<any>('/api/dms/unread-count'),
    getStats: () => this.get<any>('/api/dms/stats'),
  };

  // ---------- Notifications ----------
  public notifications = {
    get: (params?: any) => this.get<any>('/api/notifications', { params }),
    getUnread: (params?: any) => this.get<any>('/api/notifications/unread', { params }),
    markAsRead: (id: string) => this.put<any>(`/api/notifications/${id}/read`),
    markAllAsRead: () => this.put<any>('/api/notifications/read-all'),
    markMultipleAsRead: (data: any) => this.put<any>('/api/notifications/read-multiple', data),
    getUnreadCount: () => this.get<any>('/api/notifications/unread-count'),
    delete: (id: string) => this.delete<any>(`/api/notifications/${id}`),
    deleteAll: () => this.delete<any>('/api/notifications/all'),
    getGrouped: (params?: any) => this.get<any>('/api/notifications/grouped', { params }),
    getStats: () => this.get<any>('/api/notifications/stats'),
  };

  // ---------- Search ----------
  public search = {
    tweets: (params: any) => this.get<any>('/api/search/tweets', { params }),
    users: (params: any) => this.get<any>('/api/search/users', { params }),
    hashtags: (params: any) => this.get<any>('/api/search/hashtags', { params }),
    all: (params: any) => this.get<any>('/api/search/all', { params }),
    suggestions: (params: any) => this.get<any>('/api/search/suggestions', { params }),
    trending: (params?: any) => this.get<any>('/api/search/trending', { params }),
    record: (data: any) => this.post<any>('/api/search/record', data),
  };

  // ---------- Feed ----------
  public feed = {
    home: (params?: any) => this.get<any>('/api/feed/home', { params }),
    user: (username: string, params?: any) => this.get<any>(`/api/feed/user/${username}`, { params }),
    forYou: (params?: any) => this.get<any>('/api/feed/for-you', { params }),
    trending: (params?: any) => this.get<any>('/api/feed/trending', { params }),
    recommendations: (params?: any) => this.get<any>('/api/feed/recommendations', { params }),
    preferences: () => this.get<any>('/api/feed/preferences'),
    updatePreferences: (data: any) => this.put<any>('/api/feed/preferences', data),
    dismiss: (tweetId: string) => this.post<any>(`/api/feed/dismiss/${tweetId}`),
    stats: () => this.get<any>('/api/feed/stats'),
    metrics: (params?: any) => this.get<any>('/api/admin/feed/metrics', { params }),
  };

  // ---------- Admin ----------
  public admin = {
    // Users
    getUsers: (params?: any) => this.get<any>('/api/admin/users', { params }),
    getUser: (id: string) => this.get<any>(`/api/admin/users/${id}`),
    updateUser: (id: string, data: any) => this.put<any>(`/api/admin/users/${id}`, data),
    deleteUser: (id: string) => this.delete<any>(`/api/admin/users/${id}`),
    suspendUser: (id: string, data: any) => this.post<any>(`/api/admin/users/${id}/suspend`, data),
    unsuspendUser: (id: string) => this.post<any>(`/api/admin/users/${id}/unsuspend`),
    verifyUser: (id: string) => this.post<any>(`/api/admin/users/${id}/verify`),
    unverifyUser: (id: string) => this.post<any>(`/api/admin/users/${id}/unverify`),
    // Tweets
    getTweets: (params?: any) => this.get<any>('/api/admin/tweets', { params }),
    deleteTweet: (id: string) => this.delete<any>(`/api/admin/tweets/${id}`),
    // Reports
    getReports: (params?: any) => this.get<any>('/api/admin/reports', { params }),
    resolveReport: (id: string, data: any) => this.post<any>(`/api/admin/reports/${id}/resolve`, data),
    dismissReport: (id: string, data: any) => this.post<any>(`/api/admin/reports/${id}/dismiss`, data),
    // Analytics
    getAnalytics: (params?: any) => this.get<any>('/api/admin/analytics', { params }),
    getDashboard: () => this.get<any>('/api/admin/dashboard'),
  };

  // ---------- Health ----------
  public health = {
    check: () => this.get<any>('/health'),
    ready: () => this.get<any>('/ready'),
    live: () => this.get<any>('/live'),
  };
}

// ======================================================================
// Singleton Export
// ======================================================================

export const api = new ApiClient();
export default api;