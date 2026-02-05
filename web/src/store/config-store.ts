import { create } from "zustand";

export interface AppConfig {
  mode: "local" | "hosted";
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
  config: AppConfig | null;
  isLoading: boolean;
  error: string | null;
  fetchConfig: () => Promise<void>;
  isLocalMode: () => boolean;
}

const defaultHostedConfig: AppConfig = {
  mode: "hosted",
  version: "0.1.0",
  features: {
    auth: true,
    teams: true,
    apiKeys: true,
    realtime: true,
  },
};

export const useConfigStore = create<ConfigState>((set, get) => ({
  config: null,
  isLoading: true,
  error: null,

  fetchConfig: async () => {
    try {
      set({ isLoading: true, error: null });

      const response = await fetch("/api/v1/config");

      if (!response.ok) {
        // If config endpoint doesn't exist, assume hosted mode
        set({ config: defaultHostedConfig, isLoading: false });
        return;
      }

      const config = await response.json();
      set({ config, isLoading: false });
    } catch {
      // Network error or API not available - assume hosted mode
      set({ config: defaultHostedConfig, isLoading: false });
    }
  },

  isLocalMode: () => {
    const { config } = get();
    return config?.mode === "local";
  },
}));
