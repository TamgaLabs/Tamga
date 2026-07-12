"use client";

import { createContext, useContext, useState, useEffect, ReactNode } from "react";
import { me } from "./api";

type AuthState = {
  user: { user_id: number } | null;
  loading: boolean;
  login: (token: string) => Promise<void>;
  logout: () => void;
};

const AuthContext = createContext<AuthState>({
  user: null,
  loading: true,
  login: async () => {},
  logout: () => {},
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<{ user_id: number } | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem("token");
    if (token) {
      me()
        .then((u) => setUser(u))
        .catch(() => localStorage.removeItem("token"))
        .finally(() => setLoading(false));
    } else {
      setLoading(false);
    }
  }, []);

  const login = async (token: string) => {
    localStorage.setItem("token", token);
    try {
      const u = await me();
      setUser(u);
    } catch (e) {
      // A token that can't be validated is worse than none - clear it and
      // let the caller surface the failure rather than leaving a
      // half-authenticated state (token present, user null).
      localStorage.removeItem("token");
      throw e;
    }
  };

  const logout = () => {
    localStorage.removeItem("token");
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export const useAuth = () => useContext(AuthContext);
