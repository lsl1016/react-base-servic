import { createContext, useContext, type Accessor, type JSX } from 'solid-js';

export type AgentUITheme = 'light' | 'dark';

const defaultTheme: Accessor<AgentUITheme> = () => 'light';
const AgentUIThemeContext = createContext<Accessor<AgentUITheme>>(defaultTheme);

export function AgentUIThemeProvider(props: {
  theme: Accessor<AgentUITheme>;
  children: JSX.Element;
}) {
  return (
    <AgentUIThemeContext.Provider value={props.theme}>
      {props.children}
    </AgentUIThemeContext.Provider>
  );
}

export function useAgentUITheme(): Accessor<AgentUITheme> {
  return useContext(AgentUIThemeContext);
}
