import { create } from "zustand";

export interface AppConfig {
  mode: "local";
  version: string;
  logdir?: string;
  features: {
    auth: boolean;
    teams: boolean;
    apiKeys: boolean;
    realtime: boolean;
  };
}

interface ConfigState {
  config: AppConfig;
  isLoading: boolean;
  error: string | null;
  fetchConfig: () => Promise<void>;
  isLocalMode: () => boolean;
}

const defaultConfig: AppConfig = {
  mode: "local",
  version: "0.1.0",
  features: {
    auth: false,
    teams: false,
    apiKeys: false,
    realtime: false,
  },
};

export const useConfigStore = create<ConfigState>((set, get) => ({
  config: defaultConfig,
  isLoading: false,
  error: null,

  fetchConfig: async () => {
    try {
      set({ isLoading: true, error: null });

      const response = await fetch("/api/v1/config");

      if (response.ok) {
        const config = await response.json();
        set({ config: { ...defaultConfig, ...config }, isLoading: false });
      } else {
        set({ config: defaultConfig, isLoading: false });
      }
    } catch {
      set({ config: defaultConfig, isLoading: false });
    }
  },

  isLocalMode: () => true,
}));
