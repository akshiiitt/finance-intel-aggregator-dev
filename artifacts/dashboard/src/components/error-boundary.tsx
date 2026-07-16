import { Component, type ErrorInfo, type ReactNode } from "react";
import { AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

// Top-level crash guard. Without this, any unhandled render exception
// anywhere in the page tree white-screens the entire dashboard — there was
// previously no boundary anywhere in the app to catch and contain one.
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("FinanceIntel dashboard crashed:", error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex flex-col items-center justify-center h-screen gap-3 px-8 text-center bg-background">
          <AlertCircle size={22} className="text-bearish" />
          <p className="text-sm font-medium text-foreground">Something went wrong</p>
          <p className="text-sm text-muted-foreground max-w-md">
            {this.state.error.message || "The dashboard hit an unexpected error and had to stop rendering this page."}
          </p>
          <Button
            variant="outline"
            size="sm"
            className="mt-1"
            onClick={() => {
              this.setState({ error: null });
              window.location.reload();
            }}
          >
            Reload
          </Button>
        </div>
      );
    }
    return this.props.children;
  }
}
