import { Link } from "wouter";
import { Compass } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="flex-1 flex items-center justify-center">
      <div className="text-center max-w-sm px-6">
        <Compass size={28} className="mx-auto mb-4 text-muted-foreground/50" />
        <h1 className="text-xl font-semibold text-foreground mb-1.5">Page not found</h1>
        <p className="text-sm text-muted-foreground mb-5">
          The page you're looking for doesn't exist or may have moved.
        </p>
        <Button asChild size="sm">
          <Link href="/">Back to feed</Link>
        </Button>
      </div>
    </div>
  );
}
