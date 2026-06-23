"use client";

import { createContext, useContext, useState, useEffect, ReactNode } from "react";
import { me } from "./api";

type AuthState = {
  user: { user_id: number } | null;
  loading: boolean;
  login: (token: string) => void;
  logout: () => void;
};

const AuthContext = createContext<AuthState>({
  user: null,
  loading: true,
  login: () => {},
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

  const login = (token: string) => {
    localStorage.setItem("token", token);
    me().then((u) => setUser(u));
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
