/**
 * AskQuestionCard - ask_question 内置工具的作答卡片
 *
 * 状态机（跟随 ToolCallState.status）：
 * - waiting：渲染问题表单（单选/多选 + 自由输入 + 跳过），提交后通过 tool_use_answer 回填；
 * - done：回显后端加工后的最终答案（tool_use_end.content 解析）；
 * - error：展示错误信息。
 *
 * 「其他」自由输入由本组件自动附加，不来自 toolInput.questions。
 */

import { createMemo, createSignal, For, on, Show } from "solid-js";
import IconMdiCheckCircle from "~icons/mdi/check-circle";
import IconMdiChevronLeft from "~icons/mdi/chevron-left";
import IconMdiChevronRight from "~icons/mdi/chevron-right";
import IconMdiCloseCircle from "~icons/mdi/close-circle";
import IconMdiCommentQuestion from "~icons/mdi/comment-question";
import type { AskQuestionAnswerContent } from "../../protocol/types";
import type { ToolCallState } from "../../runtime/types";
import { RichInputEditor } from "../editor/RichInputEditor";
import { parseAgentInputText, serializeAgentInputParts, type AgentInputPart } from "../editor/types";

/** toolInput.questions 的结构（与后端 askQuestionToolDefinition 对齐） */
interface AskQuestion {
  id: string;
  prompt: string;
  allowMultiple?: boolean;
  options: { id: string; label: string; description?: string }[];
}

/** tool_use_end.content 的结构（与后端 askQuestionResult 对齐） */
interface AskQuestionResult {
  answers: {
    questionId: string;
    prompt: string;
    selected: { id: string; label: string }[];
    freeText?: string;
  }[];
  skipped: boolean;
  /** run 被取消/断线导致未作答；与用户主动跳过区分 */
  cancelled?: boolean;
  note?: string;
}

export interface AskQuestionCardProps {
  /** 工具调用状态 */
  toolCall: ToolCallState;
  /** 提交答案；不传时表单只读（如历史回放中 run 已结束的场景） */
  onSubmit?: (toolUseId: string, content: AskQuestionAnswerContent) => void;
  /** 是否使用简约克制样式，用于历史消息中的答案回显 */
  minimal?: boolean;
}

function parseQuestions(input: Record<string, unknown>): AskQuestion[] {
  const questions = input.questions;
  if (!Array.isArray(questions)) return [];
  return questions.filter(
    (item): item is AskQuestion =>
      !!item && typeof item === "object" && typeof (item as AskQuestion).id === "string" && Array.isArray((item as AskQuestion).options),
  );
}

function parseResult(result?: string): AskQuestionResult | null {
  if (!result) return null;
  try {
    const parsed = JSON.parse(result) as AskQuestionResult;
    return Array.isArray(parsed.answers) ? parsed : null;
  } catch {
    return null;
  }
}

/** 把原始问题列表转成未作答的回显项，兜底 answers 为空的旧数据 */
function legacyUnansweredItems(questions: AskQuestion[]): AskQuestionResult["answers"] {
  return questions.map((question) => ({ questionId: question.id, prompt: question.prompt, selected: [] }));
}

/** 只读的问题列表，用于取消态和旧数据兜底，让历史里始终能看到当时问了什么 */
function QuestionListReadonly(props: { questions: AskQuestion[]; note: string }) {
  return (
    <div class="agent-ui-ask-question-body agent-ui-ask-question-summary">
      <div class="agent-ui-ask-question-skipped">{props.note}</div>
      <For each={props.questions}>
        {(question) => (
          <div class="agent-ui-ask-question-summary-item">
            <span class="agent-ui-ask-question-summary-prompt">{question.prompt}</span>
            <span class="agent-ui-ask-question-summary-answer">未作答</span>
          </div>
        )}
      </For>
    </div>
  );
}

export function AskQuestionCard(props: AskQuestionCardProps) {
  const questions = () => parseQuestions(props.toolCall.input ?? {});
  // key: questionId, value: 已选选项 id 集合
  const [selections, setSelections] = createSignal<Record<string, string[]>>({});
  // key: questionId, value: 自由输入文本
  const [freeTexts, setFreeTexts] = createSignal<Record<string, string>>({});
  const [submitted, setSubmitted] = createSignal(false);
  const [currentIndex, setCurrentIndex] = createSignal(0);

  const toggleOption = (question: AskQuestion, optionId: string) => {
    setSelections((prev) => {
      const current = prev[question.id] ?? [];
      let next: string[];
      if (question.allowMultiple) {
        next = current.includes(optionId) ? current.filter((id) => id !== optionId) : [...current, optionId];
      } else {
        next = current.includes(optionId) ? [] : [optionId];
      }
      return { ...prev, [question.id]: next };
    });
    // 单选时，选择普通选项后清空「其他」自由输入，保持互斥
    if (!question.allowMultiple) {
      const current = selections()[question.id] ?? [];
      const willBeSelected = !current.includes(optionId);
      if (willBeSelected) {
        setFreeTexts((prev) => ({ ...prev, [question.id]: "" }));
      }
    }
  };

  const setFreeText = (question: AskQuestion, value: string) => {
    setFreeTexts((prev) => ({ ...prev, [question.id]: value }));
    // 单选情况下，用户在「其他」输入框输入内容时，视为选择「其他」，清空已选选项
    if (!question.allowMultiple && value.trim() !== "") {
      setSelections((prev) => ({ ...prev, [question.id]: [] }));
    }
  };

  const freeTextResetKey = () => currentIndex() + (submitted() ? 10000 : 0);

  const isAnswered = (question: AskQuestion) =>
    (selections()[question.id]?.length ?? 0) > 0 || (freeTexts()[question.id]?.trim() ?? "") !== "";

  const isCurrentQuestionAnswered = () => isAnswered(currentQuestion());

  const buildContent = (skipped: boolean): AskQuestionAnswerContent => ({
    answers: skipped
      ? []
      : questions().map((question) => ({
          questionId: question.id,
          selectedOptionIds: selections()[question.id] ?? [],
          freeText: (freeTexts()[question.id] ?? "").trim(),
        })),
    skipped,
  });

  const submit = (skipped: boolean) => {
    if (submitted() || !props.onSubmit) return;
    setSubmitted(true);
    props.onSubmit(props.toolCall.toolUseId, buildContent(skipped));
  };

  const handlePrimaryAction = () => {
    if (submitted() || !props.onSubmit) return;
    if (canGoNext()) {
      goNext();
    } else {
      submit(false);
    }
  };

  const interactive = () => props.toolCall.status === "waiting" && !!props.onSubmit && !submitted();
  const result = () => parseResult(props.toolCall.result);

  const isMinimal = () => !!props.minimal;
  const hasMultipleQuestions = () => questions().length > 1;
  const currentQuestion = () => questions()[currentIndex()];
  const draftForCurrentQuestion = createMemo(
    on(currentIndex, () => {
      const text = freeTexts()[currentQuestion().id] ?? "";
      return text ? { key: currentIndex(), parts: parseAgentInputText(text) } : undefined;
    }),
  );
  const canGoPrev = () => currentIndex() > 0;
  const canGoNext = () => currentIndex() < questions().length - 1;
  const goPrev = () => setCurrentIndex((i) => Math.max(0, i - 1));
  const goNext = () => setCurrentIndex((i) => Math.min(questions().length - 1, i + 1));

  return (
    <div class="agent-ui-ask-question-card" classList={{ "agent-ui-ask-question-card-minimal": isMinimal() }}>
      <div class="agent-ui-ask-question-header">
        <Show when={!isMinimal()}>
          <span class="agent-ui-ask-question-icon">
            <Show
              when={props.toolCall.status === "done"}
              fallback={
                <Show
                  when={props.toolCall.status === "error"}
                  fallback={<IconMdiCommentQuestion width="16" height="16" />}
                >
                  <IconMdiCloseCircle width="16" height="16" />
                </Show>
              }
            >
              <IconMdiCheckCircle width="16" height="16" />
            </Show>
          </span>
        </Show>
        <Show when={!isMinimal()}>
          <span class="agent-ui-ask-question-title">{props.toolCall.description || "需要你确认几个问题"}</span>
        </Show>
        <Show when={props.toolCall.status === "waiting" && !isMinimal() && !hasMultipleQuestions()}>
          <span class="agent-ui-ask-question-status">{submitted() ? "已提交，等待继续…" : "等待作答"}</span>
        </Show>
        <Show when={props.toolCall.status === "cancelled" && !isMinimal()}>
          <span class="agent-ui-ask-question-status">已取消</span>
        </Show>
        <Show when={hasMultipleQuestions() && !isMinimal()}>
          <div class="agent-ui-ask-question-nav">
            <button
              type="button"
              class="agent-ui-ask-question-nav-btn"
              disabled={!canGoPrev()}
              onClick={goPrev}
            >
              <IconMdiChevronLeft width="16" height="16" />
            </button>
            <span class="agent-ui-ask-question-nav-text">{currentIndex() + 1} / {questions().length}</span>
            <button
              type="button"
              class="agent-ui-ask-question-nav-btn"
              disabled={!canGoNext()}
              onClick={goNext}
            >
              <IconMdiChevronRight width="16" height="16" />
            </button>
          </div>
        </Show>
      </div>

      {/* 作答表单 */}
      <Show when={props.toolCall.status === "waiting"}>
        <div class="agent-ui-ask-question-body">
          <div class="agent-ui-ask-question-content">
            <Show when={hasMultipleQuestions()}>
              <div class="agent-ui-ask-question-prompt">{currentQuestion().prompt}</div>
            </Show>
            <For each={questions()}>
              {(question, index) => (
                <Show when={!hasMultipleQuestions() || index() === currentIndex()}>
                  <div class="agent-ui-ask-question-item">
                    <Show when={!hasMultipleQuestions()}>
                      <div class="agent-ui-ask-question-prompt">{question.prompt}</div>
                    </Show>
                    <div class="agent-ui-ask-question-options">
                      <For each={question.options}>
                        {(option) => (
                          <label
                            class="agent-ui-ask-question-option"
                            classList={{
                              "agent-ui-ask-question-option-selected": (selections()[question.id] ?? []).includes(option.id),
                              "agent-ui-ask-question-option-disabled": !interactive(),
                            }}
                          >
                            <span class="agent-ui-ask-question-option-row">
                              <input
                                type={question.allowMultiple ? "checkbox" : "radio"}
                                name={`${props.toolCall.toolUseId}-${question.id}`}
                                checked={(selections()[question.id] ?? []).includes(option.id)}
                                disabled={!interactive()}
                                onClick={() => toggleOption(question, option.id)}
                              />
                              <span class="agent-ui-ask-question-option-label">{option.label}</span>
                            </span>
                            <Show when={option.description}>
                              <span class="agent-ui-ask-question-option-desc" title={option.description}>{option.description}</span>
                            </Show>
                          </label>
                        )}
                      </For>
                    </div>
                  </div>
                </Show>
              )}
            </For>
          </div>

          <Show when={props.onSubmit}>
            <div class="agent-ui-ask-question-actions">
              <RichInputEditor
                placeholder="其他（自由输入或补充说明）"
                disabled={submitted() || !interactive()}
                resetKey={freeTextResetKey()}
                draft={draftForCurrentQuestion()}
                onSubmit={() => handlePrimaryAction()}
                onChange={(parts, empty) => {
                  setFreeText(currentQuestion(), empty ? "" : serializeAgentInputParts(parts));
                }}
              />
              <div class="agent-ui-ask-question-action-buttons">
                <button
                  type="button"
                  class="agent-ui-ask-question-btn agent-ui-ask-question-btn-text"
                  disabled={submitted()}
                  onClick={() => submit(true)}
                >
                  跳过
                </button>
                <button
                  type="button"
                  class="agent-ui-ask-question-btn agent-ui-ask-question-btn-primary"
                  disabled={submitted()}
                  onClick={handlePrimaryAction}
                >
                  继续
                </button>
              </div>
            </div>
          </Show>
        </div>
      </Show>

      {/* 已完成：回显最终答案；旧数据 answers 为空时回退到原始问题列表 */}
      <Show when={props.toolCall.status === "done"}>
        <Show
          when={result()}
          fallback={<QuestionListReadonly questions={questions()} note="已完成" />}
        >
          {(parsed) => (
            <div class="agent-ui-ask-question-body agent-ui-ask-question-summary" classList={{ "agent-ui-ask-question-summary-minimal": isMinimal() }}>
              <Show when={parsed().skipped}>
                <div class="agent-ui-ask-question-skipped">
                  {parsed().cancelled ? "运行已取消，未作答" : "用户跳过了提问"}
                </div>
              </Show>
              <For each={parsed().answers.length > 0 ? parsed().answers : legacyUnansweredItems(questions())}>
                {(answer) => (
                  <div class="agent-ui-ask-question-summary-item">
                    <span class="agent-ui-ask-question-summary-prompt">{answer.prompt}</span>
                    <span class="agent-ui-ask-question-summary-answer">
                      {[...answer.selected.map((item) => item.label), ...(answer.freeText ? [answer.freeText] : [])].join("、") || "未作答"}
                    </span>
                  </div>
                )}
              </For>
            </div>
          )}
        </Show>
      </Show>

      {/* run 被取消，提问未收到结果 */}
      <Show when={props.toolCall.status === "cancelled"}>
        <QuestionListReadonly questions={questions()} note="运行已取消，未作答" />
      </Show>

      {/* 出错 */}
      <Show when={props.toolCall.status === "error"}>
        <div class="agent-ui-ask-question-body agent-ui-ask-question-error">
          {props.toolCall.result || "提问失败"}
        </div>
      </Show>
    </div>
  );
}
