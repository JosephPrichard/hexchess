import type { Hex } from '$lib/api/models';

export const BadPromotion = "INCOMPLETE_PROMOTION";
export type BadPromotionType = typeof BadPromotion;

export interface MoveAction {
	from: Hex;
	to: Hex;
	promotion: Promotion;
}

export type Promotion = {piece: number, kind: number};
export const NoPromotion = {piece: 0, kind: 0};