export type UserRole = "SUPER_ADMIN" | "JUNIOR_ADMIN" | "SUBCONTRACTOR";
export type TenderStatus = "DRAFT" | "PENDING_APPROVAL" | "CHANGES_REQUESTED" | "APPROVED" | "PUBLISHED" | "CLOSED" | "AWARDED" | "IN_PROGRESS" | "COMPLETED" | "CANCELLED";
export type ContractStatus = "DRAFT" | "ACTIVE" | "PENDING_CLOSURE" | "COMPLETED" | "TERMINATED";
export type TenderResponseStatus = "SUBMITTED" | "CONFIRMATION_REQUIRED" | "CONFIRMED" | "WITHDRAWN" | "NOT_SELECTED" | "WINNER";

export type SessionUser = {
  id: number;
  email: string;
  role: UserRole;
  subcontractorId?: number;
  mustChangePassword: boolean;
  /** Used only to hide sections; the API enforces access on every request. */
  permissions?: string[];
};
